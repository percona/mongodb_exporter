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
	"sync/atomic"
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
	// client is the pooled client, nil until a connect has succeeded.
	client atomic.Pointer[pooledClient]
	// clientGroup collapses concurrent attempts to build client into one connect.
	clientGroup           singleflight.Group
	logger                *slog.Logger
	opts                  *Opts
	lock                  *sync.Mutex
	totalCollectionsCount int
}

// pooledClient is the cached client together with how many scrapes in a row have failed its
// health check.
type pooledClient struct {
	*mongo.Client

	pingFailures atomic.Int32
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
	errConnectPanicked    = errors.New("cannot connect to MongoDB: connect panicked")
)

const (
	defaultCacheSize = 1000

	// defaultConnectTimeout stands in for a connect or server-selection timeout of zero, which
	// the driver reads as no timeout at all. It equals the driver's own server-selection default.
	defaultConnectTimeout = 30 * time.Second

	// defaultMaxConnIdleTime closes pooled connections that sat idle this long. The driver never
	// prunes idle connections on its own and reuses them unchecked, so a socket a middlebox
	// dropped during a gap between scrapes would otherwise fail the next scrape.
	defaultMaxConnIdleTime = 5 * time.Minute

	// maxConsecutivePingFailures is how many scrapes in a row may fail the pooled client's
	// health check before it is dropped and built anew.
	maxConsecutivePingFailures = 3
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
	// Warm the pool so the first scrape does not pay for the connect. getClient bounds the
	// attempt and buildClient logs a failure, which every scrape retries anyway.
	if opts.GlobalConnPool {
		go func() {
			_, _ = exp.getClient(context.Background())
		}()
	}

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

	if pooled := e.client.Load(); pooled != nil {
		err := pooled.Ping(ctx, nil)
		if err == nil {
			pooled.pingFailures.Store(0)

			return pooled.Client, nil
		}

		// One failed health check is transient -- an unreachable server, a scrape out of time --
		// and the driver reconnects its pool on its own. A client that keeps failing is dropped
		// so the next scrape builds one from scratch: that is the only way to pick up what the
		// driver reads once, such as rotated TLS material.
		if pooled.pingFailures.Add(1) >= maxConsecutivePingFailures && e.client.CompareAndSwap(pooled, nil) {
			e.logger.Warn("Dropping MongoDB client after repeated failed health checks, reconnecting on next scrape", "error", err)
			_ = pooled.Disconnect(ctx)
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
	built := e.clientGroup.DoChan("", e.buildClient)

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

// buildClient fills the client cache and returns what is in it. It runs as a singleflight
// flight, so at most one of these is in progress at a time.
func (e *Exporter) buildClient() (_ any, err error) {
	// singleflight re-raises a flight's panic on a goroutine of its own, where nothing recovers
	// it, so a connect that panicked would take the process down rather than fail one scrape.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", errConnectPanicked, r)
		}
	}()

	// singleflight retires its key once a flight returns, so a scrape that read an empty
	// cache and arrives after an earlier flight finished starts a new flight rather than
	// joining the old one. It must not displace a live client, which nothing ever disconnects.
	if pooled := e.client.Load(); pooled != nil {
		return pooled.Client, nil
	}

	// Resolved here rather than on the caller's goroutine: for mongodb+srv:// this performs
	// SRV and TXT lookups through net.LookupSRV, which takes no context, so a slow resolver
	// can only delay the pool, not a scrape's response.
	clientOpts, connectTimeout, err := clientOptionsFor(e.opts)
	if err != nil {
		return nil, err
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	newClient, err := connectWith(connectCtx, clientOpts)
	if err != nil {
		// Every scrape waiting on this flight may have given up on its own deadline already, in
		// which case nobody else sees the cause.
		e.logger.Error("MongoDB connect attempt failed", "error", err)

		return nil, err
	}

	e.client.Store(&pooledClient{Client: newClient})

	return newClient, nil
}

// clientOptionsFor builds the driver options for opts and returns, alongside them, the
// budget one connect attempt gets. The driver spends its connect and server-selection
// timeouts in sequence, so the budget is their sum, and a caller that bounds the attempt
// with it never cuts short a wait the operator configured.
//
// connectTimeoutMS in the URI wins over --mongodb.connect-timeout-ms; a zero there is the
// driver's "no dial timeout" and is passed through. Server selection is taken from the URI,
// else derived from the flag's connect timeout, else the driver's default. A zero selection
// timeout, or a zero flag, would mean no timeout at all and is replaced by
// defaultConnectTimeout: a connect has to finish for the pool to ever fill.
func clientOptionsFor(opts *Opts) (*options.ClientOptions, time.Duration, error) {
	clientOpts, err := dsn_fix.ClientOptionsForDSN(opts.URI)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid dsn: %w", err)
	}

	clientOpts.SetDirect(opts.DirectConnect)
	clientOpts.SetAppName("mongodb_exporter")

	if clientOpts.MaxConnIdleTime == nil {
		clientOpts.SetMaxConnIdleTime(defaultMaxConnIdleTime)
	}

	connectTimeout := defaultConnectTimeout
	if opts.ConnectTimeoutMS > 0 {
		connectTimeout = time.Duration(opts.ConnectTimeoutMS) * time.Millisecond
	}

	connectFromURI := clientOpts.ConnectTimeout != nil
	if connectFromURI {
		connectTimeout = *clientOpts.ConnectTimeout
	} else {
		clientOpts.SetConnectTimeout(connectTimeout)
	}

	selectionTimeout := defaultConnectTimeout
	switch {
	case clientOpts.ServerSelectionTimeout != nil && *clientOpts.ServerSelectionTimeout > 0:
		selectionTimeout = *clientOpts.ServerSelectionTimeout
	case !connectFromURI:
		selectionTimeout = connectTimeout
	}
	clientOpts.SetServerSelectionTimeout(selectionTimeout)

	return clientOpts, connectTimeout + selectionTimeout, nil
}

func connect(ctx context.Context, opts *Opts) (*mongo.Client, error) {
	clientOpts, _, err := clientOptionsFor(opts)
	if err != nil {
		return nil, err
	}

	return connectWith(ctx, clientOpts)
}

// connectWith is connect for a caller that already holds resolved options.
func connectWith(ctx context.Context, clientOpts *options.ClientOptions) (*mongo.Client, error) {
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
