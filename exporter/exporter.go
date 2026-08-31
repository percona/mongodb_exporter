// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package exporter implements the collectors and metrics handlers.
package exporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/singleflight"

	"github.com/percona/mongodb_exporter/exporter/dsn_fix"
)

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	client *mongo.Client
	// clientMu guards the client pointer only. It is never held across a command, so
	// concurrent scrapes of this target do not wait for each other.
	clientMu sync.RWMutex
	// clientGroup collapses concurrent attempts to build client into one connect.
	clientGroup           singleflight.Group
	logger                *slog.Logger
	opts                  *Opts
	lock                  *sync.Mutex
	totalCollectionsCount int
}

// Opts holds new exporter options.
type Opts struct {
	CompatibleMode         bool
	DirectConnect          bool
	ConnectTimeoutMS       int
	DisableDefaultRegistry bool
	DiscoveringMode        bool
	GlobalConnPool         bool
	TimeoutOffset          int

	CollectAll                     bool
	EnableDBStats                  bool
	EnableDBStatsFreeStorage       bool
	EnableDiagnosticData           bool
	EnableDiagnosticDataHistograms bool
	EnableReplicasetStatus         bool
	EnableReplicasetConfig         bool
	EnableCurrentopMetrics         bool
	EnableTopMetrics               bool
	EnableIndexStats               bool
	EnableCollStats                bool
	EnableProfile                  bool
	EnableShards                   bool
	EnableFCV                      bool // Feature Compatibility Version.
	EnablePBMMetrics               bool

	EnableOverrideDescendingIndex bool

	// Only get stats for the collections matching this list of namespaces.
	// Example: db1.col1,db.col1
	CollStatsNamespaces    []string
	CollStatsLimit         int
	CollStatsEnableDetails bool
	IndexStatsCollections  []string
	CurrentOpSlowTime      string
	ProfileTimeTS          int

	Logger *slog.Logger

	URI      string
	NodeName string
}

var (
	errCannotHandleType   = fmt.Errorf("don't know how to handle data type")
	errUnexpectedDataType = fmt.Errorf("unexpected data type")
)

const (
	defaultCacheSize = 1000

	// defaultConnectTimeout bounds a connect when neither the URI nor Opts.ConnectTimeoutMS
	// gives it a budget. The driver would otherwise be handed a server-selection timeout of
	// 0, which it reads as no timeout at all, and an unanswering server would block a
	// connect for the lifetime of the process.
	defaultConnectTimeout = 30 * time.Second
)

// New connects to the database and returns a new Exporter instance.
func New(opts *Opts) *Exporter {
	if opts == nil {
		opts = new(Opts)
	}

	if opts.Logger == nil {
		promslogConfig := &promslog.Config{}
		opts.Logger = promslog.New(promslogConfig)
	}

	exp := &Exporter{
		logger:                opts.Logger,
		opts:                  opts,
		lock:                  &sync.Mutex{},
		totalCollectionsCount: -1, // Not calculated yet. waiting the db connection.
	}
	// Try initial connect. Connection will be retried with every scrape.
	// getClient bounds the connect itself, so no deadline is imposed here.
	go func() {
		ctx := context.Background()

		client, err := exp.getClient(ctx)
		if err != nil {
			exp.logger.Error("Cannot connect to MongoDB", "error", err)

			return
		}

		// With the global pool this client is the one every later scrape reuses. Otherwise
		// nothing owns it, since each scrape builds its own, so leaving it connected would
		// leak a topology, its monitoring goroutines and a connection.
		if exp.opts.GlobalConnPool {
			return
		}

		err = client.Disconnect(ctx)
		if err != nil {
			exp.logger.Error("Cannot disconnect client", "error", err)
		}
	}()

	return exp
}

func (e *Exporter) getTotalCollectionsCount() int {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.totalCollectionsCount
}

func (e *Exporter) makeRegistry(ctx context.Context, client *mongo.Client, topologyInfo labelsGetter, requestOpts Opts) *prometheus.Registry {
	registry := prometheus.NewRegistry()

	nodeType, err := getNodeType(ctx, client)
	if err != nil {
		e.logger.Error("Registry - Cannot get node type", "error", err)
	}

	dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, e.logger.With("component", "buildInfo"))
	if err != nil {
		e.logger.Warn("Registry - Cannot get MongoDB buildInfo", "error", err)
	}

	gc := newGeneralCollector(ctx, client, nodeType, e.opts.Logger)
	registry.MustRegister(gc)

	// Enable collectors like collstats and indexstats depending on the number of collections
	// present in the database.
	limitsOk := false
	if e.opts.CollStatsLimit <= 0 || // Unlimited
		e.getTotalCollectionsCount() <= e.opts.CollStatsLimit {
		limitsOk = true
	}

	if e.opts.CollectAll {
		if len(e.opts.CollStatsNamespaces) == 0 {
			e.opts.DiscoveringMode = true
		}
		e.opts.EnableDiagnosticData = true
		e.opts.EnableDBStats = true
		e.opts.EnableDBStatsFreeStorage = true
		e.opts.EnableCollStats = true
		e.opts.EnableTopMetrics = true
		e.opts.EnableReplicasetStatus = true
		e.opts.EnableReplicasetConfig = true
		e.opts.EnableIndexStats = true
		e.opts.EnableCurrentopMetrics = true
		e.opts.EnableProfile = true
		e.opts.EnableShards = true
		e.opts.EnableFCV = true
		e.opts.EnablePBMMetrics = true
	}

	// arbiter only have isMaster privileges
	if nodeType == typeArbiter {
		e.opts.EnableDBStats = false
		e.opts.EnableDBStatsFreeStorage = false
		e.opts.EnableCollStats = false
		e.opts.EnableTopMetrics = false
		e.opts.EnableReplicasetStatus = false
		e.opts.EnableIndexStats = false
		e.opts.EnableCurrentopMetrics = false
		e.opts.EnableProfile = false
		e.opts.EnableShards = false
		e.opts.EnableFCV = false
		e.opts.EnablePBMMetrics = false
	}

	// If we manually set the collection names we want or auto discovery is set.
	if (len(e.opts.CollStatsNamespaces) > 0 || e.opts.DiscoveringMode) && e.opts.EnableCollStats && limitsOk && requestOpts.EnableCollStats {
		cc := newCollectionStatsCollector(ctx, client, e.opts.Logger,
			e.opts.DiscoveringMode,
			topologyInfo, e.opts.CollStatsNamespaces, e.opts.CollStatsEnableDetails)
		registry.MustRegister(cc)
	}

	// If we manually set the collection names we want or auto discovery is set.
	if (len(e.opts.IndexStatsCollections) > 0 || e.opts.DiscoveringMode) && e.opts.EnableIndexStats && limitsOk && requestOpts.EnableIndexStats {
		ic := newIndexStatsCollector(ctx, client, e.opts.Logger,
			e.opts.DiscoveringMode, e.opts.EnableOverrideDescendingIndex,
			topologyInfo, e.opts.IndexStatsCollections)
		registry.MustRegister(ic)
	}

	if e.opts.EnableDiagnosticData && requestOpts.EnableDiagnosticData {
		ddc := newDiagnosticDataCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, topologyInfo, dbBuildInfo, e.opts.EnableDiagnosticDataHistograms)
		registry.MustRegister(ddc)
	}

	if e.opts.EnableDBStats && limitsOk && requestOpts.EnableDBStats {
		cc := newDBStatsCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, topologyInfo, nil, e.opts.EnableDBStatsFreeStorage)
		registry.MustRegister(cc)
	}

	if e.opts.EnableCurrentopMetrics && nodeType != typeMongos && requestOpts.EnableCurrentopMetrics {
		coc := newCurrentopCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, topologyInfo, e.opts.CurrentOpSlowTime)
		registry.MustRegister(coc)
	}

	if e.opts.EnableProfile && nodeType != typeMongos && limitsOk && requestOpts.EnableProfile && e.opts.ProfileTimeTS != 0 {
		pc := newProfileCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, topologyInfo, e.opts.ProfileTimeTS)
		registry.MustRegister(pc)
	}

	if e.opts.EnableTopMetrics && nodeType != typeMongos && limitsOk && requestOpts.EnableTopMetrics {
		tc := newTopCollector(ctx, client, e.opts.Logger, topologyInfo)
		registry.MustRegister(tc)
	}

	// replSetGetStatus is not supported through mongos.
	if e.opts.EnableReplicasetStatus && nodeType != typeMongos && requestOpts.EnableReplicasetStatus {
		rsgsc := newReplicationSetStatusCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, topologyInfo)
		registry.MustRegister(rsgsc)
	}

	// replSetGetStatus is not supported through mongos.
	if e.opts.EnableReplicasetConfig && nodeType != typeMongos && requestOpts.EnableReplicasetConfig {
		rsgsc := newReplicationSetConfigCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, topologyInfo)
		registry.MustRegister(rsgsc)
	}
	if e.opts.EnableShards && nodeType == typeMongos && requestOpts.EnableShards {
		sc := newShardsCollector(ctx, client, e.opts.Logger, e.opts.CompatibleMode)
		registry.MustRegister(sc)
	}

	if e.opts.EnableFCV && nodeType != typeMongos {
		fcvc := newFeatureCompatibilityCollector(ctx, client, e.opts.Logger)
		registry.MustRegister(fcvc)
	}

	if e.opts.EnablePBMMetrics && requestOpts.EnablePBMMetrics {
		pbmc := newPbmCollector(ctx, client, e.opts.URI, e.opts.Logger)
		registry.MustRegister(pbmc)
	}

	return registry
}

// getClient returns a client to scrape with. Everything below keeps to one rule: a scrape
// must come back within its own budget, so that a struggling MongoDB is reported as
// mongodb_up 0 rather than as a scrape Prometheus never gets an answer to.
func (e *Exporter) getClient(ctx context.Context) (*mongo.Client, error) {
	if !e.opts.GlobalConnPool {
		// Create a new client for every scrape. The caller disconnects it.
		return connect(ctx, e.opts)
	}

	client := e.cachedClient()

	// Health-check outside the lock. Holding it across the Ping would make concurrent
	// scrapes of this target queue behind each other, letting one slow scrape push the
	// next past its budget.
	if client != nil {
		err := client.Ping(ctx, nil)
		if err == nil {
			return client, nil
		}

		// A disconnected client never recovers, so forget it and let the next scrape build a
		// new one. Every other error is transient -- an unreachable server, a scrape that ran
		// out of time -- and the driver reconnects the pool on its own; tearing it down would
		// only add churn while MongoDB is already struggling.
		if errors.Is(err, mongo.ErrClientDisconnected) {
			e.logger.Warn("Dropping disconnected MongoDB client, reconnecting on next scrape")
			e.clientMu.Lock()
			if e.client == client {
				e.client = nil
			}
			e.clientMu.Unlock()
		}

		return nil, fmt.Errorf("cannot connect to MongoDB: %w", err)
	}

	// Build the client. Initialization is retried with every scrape until it succeeds once.
	//
	// The connect gets its own budget rather than the scrape's: a scrape shorter than one
	// connect would cancel every attempt and leave the pool permanently empty, so every
	// scrape would keep paying for a connect that can never finish. Concurrent scrapes
	// collapse onto that one connect, and each gives up on its own deadline while it
	// carries on in the background.
	//
	// The budget comes from clientOptionsFor rather than the flag alone, so it matches what
	// the driver itself was given. A deadline shorter than that would abort a handshake
	// still within its own limit, on every scrape, leaving mongodb_up at 0 for good.
	_, connectTimeout, err := clientOptionsFor(e.opts)
	if err != nil {
		return nil, err
	}

	built := e.clientGroup.DoChan("", func() (any, error) { //nolint:contextcheck
		connectCtx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()

		newClient, err := connect(connectCtx, e.opts)
		if err != nil {
			return nil, err
		}

		e.clientMu.Lock()
		e.client = newClient
		e.clientMu.Unlock()

		return newClient, nil
	})

	select {
	case res := <-built:
		if res.Err != nil {
			return nil, res.Err
		}

		return res.Val.(*mongo.Client), nil //nolint:forcetypeassert
	case <-ctx.Done():
		return nil, fmt.Errorf("cannot connect to MongoDB: %w", ctx.Err())
	}
}

// Handler returns an http.Handler that serves metrics. Can be used instead of
// run for hooking up custom HTTP servers.
func (e *Exporter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scrapeTimeoutHeader := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds")
		seconds := 10.0

		if scrapeTimeoutHeader != "" {
			if parsedSeconds, err := strconv.ParseFloat(scrapeTimeoutHeader, 64); err == nil {
				seconds = parsedSeconds
			} else {
				e.logger.Info("Invalid X-Prometheus-Scrape-Timeout-Seconds header", "error", err)
			}
		}
		seconds -= float64(e.opts.TimeoutOffset)

		var client *mongo.Client
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds*float64(time.Second)))
		defer cancel()

		requestOpts := GetRequestOpts(r.URL.Query()["collect[]"], e.opts)

		client, err := e.getClient(ctx)
		if err != nil {
			e.logger.Error("Cannot connect to MongoDB", "error", err)
		}

		if client != nil && e.getTotalCollectionsCount() <= 0 {
			count, err := nonSystemCollectionsCount(ctx, client, nil, nil)
			if err == nil {
				e.lock.Lock()
				e.totalCollectionsCount = count
				e.lock.Unlock()
			}
		}

		// Close client after usage.
		if !e.opts.GlobalConnPool {
			defer func() {
				if client != nil {
					err := client.Disconnect(ctx)
					if err != nil {
						e.logger.Error("Cannot disconnect client", "error", err)
					}
				}
			}()
		}

		var gatherers prometheus.Gatherers

		if !e.opts.DisableDefaultRegistry {
			gatherers = append(gatherers, prometheus.DefaultGatherer)
		}

		var registry *prometheus.Registry
		var ti *topologyInfo
		if client != nil {
			// Topology can change between requests, so we need to get it every time.
			ti = newTopologyInfo(ctx, client, e.logger)
			registry = e.makeRegistry(ctx, client, ti, requestOpts)
		} else {
			registry = prometheus.NewRegistry()
			gc := newGeneralCollector(ctx, client, "", e.opts.Logger)
			registry.MustRegister(gc)
		}

		gatherers = append(gatherers, registry)

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog:      newHTTPErrorLogger(e.logger),
		})

		h.ServeHTTP(w, r)
	})
}

// cachedClient returns the pooled client, or nil if none has been built yet. The connect
// that fills the cache runs detached from the scrape that started it, so a caller that
// gave up on its deadline has no ordering against that write. This is the only safe way
// to read the pointer.
func (e *Exporter) cachedClient() *mongo.Client {
	e.clientMu.RLock()
	defer e.clientMu.RUnlock()

	return e.client
}

// GetRequestOpts makes exporter.Opts structure from request filters and default options.
func GetRequestOpts(filters []string, defaultOpts *Opts) Opts {
	requestOpts := Opts{}

	if len(filters) == 0 {
		requestOpts = *defaultOpts
	}

	for _, filter := range filters {
		switch filter {
		case "diagnosticdata":
			requestOpts.EnableDiagnosticData = true
		case "replicasetstatus":
			requestOpts.EnableReplicasetStatus = true
		case "replicasetconfig":
			requestOpts.EnableReplicasetConfig = true
		case "dbstats":
			requestOpts.EnableDBStats = true
		case "topmetrics":
			requestOpts.EnableTopMetrics = true
		case "currentopmetrics":
			requestOpts.EnableCurrentopMetrics = true
		case "indexstats":
			requestOpts.EnableIndexStats = true
		case "collstats":
			requestOpts.EnableCollStats = true
		case "profile":
			requestOpts.EnableProfile = true
		case "shards":
			requestOpts.EnableShards = true
		case "fcv":
			requestOpts.EnableFCV = true
		case "pbm":
			requestOpts.EnablePBMMetrics = true
		}
	}

	return requestOpts
}

// clientOptionsFor builds the driver options for opts and returns, alongside them, the
// budget one connect attempt gets. It is the single authority for that budget, so a caller
// that bounds the attempt with a context uses the same value the driver was given and can
// never cut a handshake short of what was asked for.
//
// A connectTimeoutMS in the URI wins over --mongodb.connect-timeout-ms, being the more
// specific instruction; with neither usable the budget falls back to defaultConnectTimeout,
// since the driver reads 0 as no timeout at all.
func clientOptionsFor(opts *Opts) (*options.ClientOptions, time.Duration, error) {
	clientOpts, err := dsn_fix.ClientOptionsForDSN(opts.URI)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid dsn: %w", err)
	}

	clientOpts.SetDirect(opts.DirectConnect)
	clientOpts.SetAppName("mongodb_exporter")

	budget := defaultConnectTimeout

	switch {
	case clientOpts.ConnectTimeout != nil && *clientOpts.ConnectTimeout > 0:
		budget = *clientOpts.ConnectTimeout
	case opts.ConnectTimeoutMS > 0:
		budget = time.Duration(opts.ConnectTimeoutMS) * time.Millisecond
	}

	if clientOpts.ConnectTimeout == nil {
		clientOpts.SetConnectTimeout(budget)
	}

	// Set only when the URI is silent: an explicit serverSelectionTimeoutMS is the
	// operator's instruction and outranks the budget. Left unset the driver falls back to
	// 30s, which nobody asked for and which the caller's deadline then truncates.
	if clientOpts.ServerSelectionTimeout == nil {
		clientOpts.SetServerSelectionTimeout(budget)
	}

	return clientOpts, budget, nil
}

func connect(ctx context.Context, opts *Opts) (*mongo.Client, error) {
	clientOpts, _, err := clientOptionsFor(opts)
	if err != nil {
		return nil, err
	}

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
