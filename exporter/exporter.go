// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package exporter implements the collectors and metrics handlers.
package exporter

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	path                  string
	client                *mongo.Client
	clientMu              sync.Mutex
	logger                *logrus.Logger
	opts                  *Opts
	webListenAddress      string
	lock                  *sync.Mutex
	totalCollectionsCount int
}

// Opts holds new exporter options.
type Opts struct {
	CollStatsCollections    []string
	CollStatsLimit          int
	CollectorTopMetrics     bool
	CompatibleMode          bool
	DirectConnect           bool
	DisableDefaultRegistry  bool
	DisableDiagnosticData   bool
	DisableReplicasetStatus bool
	DiscoveringMode         bool
	EnableDBStats           bool
	EnableTop               bool
	GlobalConnPool          bool
	IndexStatsCollections   []string
	Logger                  *logrus.Logger
	Path                    string
	URI                     string
	WebListenAddress        string
}

var (
	errCannotHandleType   = fmt.Errorf("don't know how to handle data type")
	errUnexpectedDataType = fmt.Errorf("unexpected data type")
)

// New connects to the database and returns a new Exporter instance.
func New(opts *Opts) *Exporter {
	if opts == nil {
		opts = new(Opts)
	}

	if opts.Logger == nil {
		opts.Logger = logrus.New()
	}

	ctx := context.Background()

	exp := &Exporter{
		path:                  opts.Path,
		logger:                opts.Logger,
		opts:                  opts,
		webListenAddress:      opts.WebListenAddress,
		lock:                  &sync.Mutex{},
		totalCollectionsCount: -1, // not calculated yet. waiting the db connection.
	}
	// Try initial connect. Connection will be retried with every scrape.
	go func() {
		_, err := exp.getClient(ctx)
		if err != nil {
			exp.logger.Errorf("Cannot connect to MongoDB: %v", err)
		}
	}()

	return exp
}

func (e *Exporter) getTotalCollectionsCount() int {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.totalCollectionsCount
}

func (e *Exporter) makeRegistry(ctx context.Context, client *mongo.Client, topologyInfo labelsGetter) *prometheus.Registry {
	registry := prometheus.NewRegistry()

	gc := generalCollector{
		ctx:    ctx,
		client: client,
		logger: e.opts.Logger,
	}
	registry.MustRegister(&gc)

	nodeType, err := getNodeType(ctx, client)
	if err != nil {
		e.logger.Errorf("Cannot get node type to check if this is a mongos: %s", err)
	}

	// enable collection dependant collectors like collstats and indexstats
	enableCollStats := false
	if e.opts.CollStatsLimit == -1 {
		enableCollStats = true
	}
	if e.getTotalCollectionsCount() > 0 && e.getTotalCollectionsCount() < e.opts.CollStatsLimit {
		enableCollStats = true
	}

	// if we manually set the collection names we want or auto discovery is set
	if (len(e.opts.CollStatsCollections) > 0 || e.opts.DiscoveringMode) && enableCollStats {
		cc := collstatsCollector{
			ctx:             ctx,
			client:          client,
			collections:     e.opts.CollStatsCollections,
			compatibleMode:  e.opts.CompatibleMode,
			discoveringMode: e.opts.DiscoveringMode,
			logger:          e.opts.Logger,
			topologyInfo:    topologyInfo,
		}
		registry.MustRegister(&cc)
	}

	// if we manually set the collection names we want or auto discovery is set
	if (len(e.opts.IndexStatsCollections) > 0 || e.opts.DiscoveringMode) && enableCollStats {
		ic := indexstatsCollector{
			ctx:             ctx,
			client:          client,
			collections:     e.opts.IndexStatsCollections,
			discoveringMode: e.opts.DiscoveringMode,
			logger:          e.opts.Logger,
			topologyInfo:    topologyInfo,
		}
		registry.MustRegister(&ic)
	}

	if !e.opts.DisableDiagnosticData {
		ddc := diagnosticDataCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&ddc)
	}

	if e.opts.EnableDBStats {
		cc := dbstatsCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&cc)
	}

	if e.opts.CollectorTopMetrics && nodeType != typeMongos {
		tc := topCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&tc)
	}

	// replSetGetStatus is not supported through mongos
	if !e.opts.DisableReplicasetStatus && nodeType != typeMongos {
		rsgsc := replSetGetStatusCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&rsgsc)
	}

	return registry
}

func (e *Exporter) getClient(ctx context.Context) (*mongo.Client, error) {
	if e.opts.GlobalConnPool {
		// get global client. Maybe it must be initialized first.
		// Initialization is retried with every scrape until it succeeds once.
		e.clientMu.Lock()
		defer e.clientMu.Unlock()

		// if client is already initialized, return it
		if e.client != nil {
			return e.client, nil
		}

		client, err := connect(ctx, e.opts.URI, e.opts.DirectConnect)
		if err != nil {
			return nil, err
		}
		e.client = client

		return client, nil
	}

	// !e.opts.GlobalConnPool: create new client for every scrape
	client, err := connect(ctx, e.opts.URI, e.opts.DirectConnect)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Handler returns an http.Handler that serves metrics. Can be used instead of
// Run for hooking up custom HTTP servers.
func (e *Exporter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var client *mongo.Client
		ctx := r.Context()

		client, err := e.getClient(ctx)
		if err != nil {
			e.logger.Errorf("Cannot connect to MongoDB: %v", err)
			http.Error(
				w,
				"An error has occurred while connecting to MongoDB:\n\n"+err.Error(),
				http.StatusInternalServerError,
			)

			return
		}

		if e.getTotalCollectionsCount() < 0 {
			count, err := allCollectionsCount(ctx, client, nil)
			if err == nil {
				e.lock.Lock()
				e.totalCollectionsCount = count
				e.lock.Unlock()
			}
		}

		// Close client after usage
		if !e.opts.GlobalConnPool {
			defer func() {
				err := client.Disconnect(ctx)
				if err != nil {
					e.logger.Errorf("Cannot disconnect client: %v", err)
				}
			}()
		}

		// topology can change between requests, so we need to get it every time
		topologyInfo, err := newTopologyInfo(ctx, client)
		if err != nil {
			e.logger.Errorf("Cannot get topology info: %v", err)
			http.Error(
				w,
				"An error has occurred while getting topology info:\n\n"+err.Error(),
				http.StatusInternalServerError,
			)

			return
		}

		registry := e.makeRegistry(ctx, client, topologyInfo)

		var gatherers prometheus.Gatherers

		if !e.opts.DisableDefaultRegistry {
			gatherers = append(gatherers, prometheus.DefaultGatherer)
		}
		gatherers = append(gatherers, registry)

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog:      e.logger,
		})

		h.ServeHTTP(w, r)
	})
}

// Run starts the exporter.
func (e *Exporter) Run() {
	handler := e.Handler()
	exporter_shared.RunServer("MongoDB", e.webListenAddress, e.path, handler)
}

func connect(ctx context.Context, dsn string, directConnect bool) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(dsn)
	clientOpts.SetDirect(directConnect)
	clientOpts.SetAppName("mongodb_exporter")

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("invalid MongoDB options: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		// Ping failed. Close background connections. Error is ignored since the ping error is more relevant.
		_ = client.Disconnect(ctx)

		return nil, fmt.Errorf("cannot connect to MongoDB: %w", err)
	}

	return client, nil
}
