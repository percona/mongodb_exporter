// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/mongo"

	commoncollector "github.com/percona/mongodb_exporter/collector/common"
	"github.com/percona/mongodb_exporter/collector/mongod"
	"github.com/percona/mongodb_exporter/collector/mongos"
	"github.com/percona/mongodb_exporter/shared"
)

const namespace = "mongodb"

// MongodbCollectorOpts is the options of the mongodb collector.
type MongodbCollectorOpts struct {
	URI                      string
	CollectDatabaseMetrics   bool
	CollectCollectionMetrics bool
	CollectTopMetrics        bool
	CollectIndexUsageStats   bool
	CollectConnPoolStats     bool
}

func (in *MongodbCollectorOpts) toSessionOps() *shared.MongoSessionOpts {
	return &shared.MongoSessionOpts{
		URI: in.URI,
	}
}

// MongodbCollector is in charge of collecting mongodb's metrics.
type MongodbCollector struct {
	Opts *MongodbCollectorOpts

	scrapesTotal              prometheus.Counter
	scrapeErrorsTotal         prometheus.Counter
	lastScrapeError           prometheus.Gauge
	lastScrapeDurationSeconds prometheus.Gauge
	mongoUp                   prometheus.Gauge

	mongoSessLock sync.Mutex
	mongoClient   *mongo.Client
}

// NewMongodbCollector returns a new instance of a MongodbCollector.
func NewMongodbCollector(opts *MongodbCollectorOpts) *MongodbCollector {
	exporter := &MongodbCollector{
		Opts: opts,

		scrapesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrapes_total",
			Help:      "Total number of times MongoDB was scraped for metrics.",
		}),
		scrapeErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a MongoDB.",
		}),
		lastScrapeError: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from MongoDB resulted in an error (1 for error, 0 for success).",
		}),
		lastScrapeDurationSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from MongoDB.",
		}),
		mongoUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether MongoDB is up.",
		}),
	}

	return exporter
}

// getClient returns the *mongo.Client or creates a new session and returns it.
// Use sync.Mutex to avoid race condition around session creation.
func (exporter *MongodbCollector) getClient() *mongo.Client {
	exporter.mongoSessLock.Lock()
	defer exporter.mongoSessLock.Unlock()

	if exporter.mongoClient == nil {
		exporter.mongoClient = shared.MongoClient(exporter.Opts.toSessionOps())
	}
	if exporter.mongoClient == nil {
		return nil
	}
	return exporter.mongoClient
}

// Close cleanly closes the mongo session if it exists.
func (exporter *MongodbCollector) Close() {
	exporter.mongoSessLock.Lock()
	defer exporter.mongoSessLock.Unlock()

	if exporter.mongoClient != nil {
		_ = exporter.mongoClient.Disconnect(context.TODO())
	}
}

// Describe sends the super-set of all possible descriptors of metrics collected by this Collector
// to the provided channel and returns once the last descriptor has been sent.
// Part of prometheus.Collector interface.
func (exporter *MongodbCollector) Describe(ch chan<- *prometheus.Desc) {
	// We cannot know in advance what metrics the exporter will generate
	// from MongoDB. So we use the poor man's describe method: Run a collect
	// and send the descriptors of all the collected metrics. The problem
	// here is that we need to connect to the MongoDB. If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored MongoDB instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	exporter.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect is called by the Prometheus registry when collecting metrics.
// Part of prometheus.Collector interface.
func (exporter *MongodbCollector) Collect(ch chan<- prometheus.Metric) {
	exporter.scrape(ch)

	exporter.scrapesTotal.Collect(ch)
	exporter.scrapeErrorsTotal.Collect(ch)
	exporter.lastScrapeError.Collect(ch)
	exporter.lastScrapeDurationSeconds.Collect(ch)
	exporter.mongoUp.Collect(ch)
}

func (exporter *MongodbCollector) scrape(ch chan<- prometheus.Metric) {
	exporter.scrapesTotal.Inc()
	var err error
	defer func(begun time.Time) {
		exporter.lastScrapeDurationSeconds.Set(time.Since(begun).Seconds())
		if err == nil {
			exporter.lastScrapeError.Set(0)
		} else {
			exporter.scrapeErrorsTotal.Inc()
			exporter.lastScrapeError.Set(1)
		}
	}(time.Now())

	mongoSess := exporter.getClient()
	if mongoSess == nil {
		err = fmt.Errorf("Can't create mongo session to %s", shared.RedactMongoUri(exporter.Opts.URI))
		log.Error(err)
		exporter.mongoUp.Set(0)
		return
	}

	var serverVersion string
	serverVersion, err = shared.MongoSessionServerVersion(mongoSess)
	if err != nil {
		log.Errorf("Problem gathering the mongo server version: %s", err)
		exporter.mongoUp.Set(0)
		return
	}
	exporter.mongoUp.Set(1)

	var nodeType string
	nodeType, err = shared.MongoSessionNodeType(mongoSess)
	if err != nil {
		log.Errorf("Problem gathering the mongo node type: %s", err)
		return
	}

	log.Debugf("Connected to: %s (node type: %s, server version: %s)", shared.RedactMongoUri(exporter.Opts.URI), nodeType, serverVersion)
	switch {
	case nodeType == "mongos":
		exporter.collectMongos(mongoSess, ch)
	case nodeType == "mongod":
		exporter.collectMongod(mongoSess, ch)
	case nodeType == "replset":
		exporter.collectMongodReplSet(mongoSess, ch)
	default:
		err = fmt.Errorf("Unrecognized node type %s", nodeType)
		log.Error(err)
	}
}

func (exporter *MongodbCollector) collectMongos(client *mongo.Client, ch chan<- prometheus.Metric) {
	log.Debug("Collecting Server Status")
	serverStatus := mongos.GetServerStatus(client)
	if serverStatus != nil {
		serverStatus.Export(ch)
	}

	log.Debug("Collecting Sharding Status")
	shardingStatus := mongos.GetShardingStatus(client)
	if shardingStatus != nil {
		shardingStatus.Export(ch)
	}

	if exporter.Opts.CollectDatabaseMetrics {
		log.Debug("Collecting Database Status From Mongos")
		dbStatList := mongos.GetDatabaseStatList(client)
		if dbStatList != nil {
			dbStatList.Export(ch)
		}
	}

	if exporter.Opts.CollectCollectionMetrics {
		log.Debug("Collecting Collection Status From Mongos")
		collStatList := mongos.GetCollectionStatList(client)
		if collStatList != nil {
			collStatList.Export(ch)
		}
	}

	if exporter.Opts.CollectConnPoolStats {
		log.Debug("Collecting ConnPoolStats Metrics")
		connPoolStats := commoncollector.GetConnPoolStats(client)
		if connPoolStats != nil {
			connPoolStats.Export(ch)
		}
	}
}

func (exporter *MongodbCollector) collectMongod(client *mongo.Client, ch chan<- prometheus.Metric) {
	log.Debug("Collecting Server Status")
	serverStatus := mongod.GetServerStatus(client)
	if serverStatus != nil {
		serverStatus.Export(ch)
	}

	if exporter.Opts.CollectDatabaseMetrics {
		log.Debug("Collecting Database Status From Mongod")
		dbStatList := mongod.GetDatabaseStatList(client)
		if dbStatList != nil {
			dbStatList.Export(ch)
		}
	}

	if exporter.Opts.CollectCollectionMetrics {
		log.Debug("Collecting Collection Status From Mongod")
		collStatList := mongod.GetCollectionStatList(client)
		if collStatList != nil {
			collStatList.Export(ch)
		}
	}

	if exporter.Opts.CollectTopMetrics {
		log.Debug("Collecting Top Metrics")
		topStatus := mongod.GetTopStatus(client)
		if topStatus != nil {
			topStatus.Export(ch)
		}
	}

	if exporter.Opts.CollectIndexUsageStats {
		log.Debug("Collecting Index Statistics")
		indexStatList := mongod.GetIndexUsageStatList(client)
		if indexStatList != nil {
			indexStatList.Export(ch)
		}
	}

	if exporter.Opts.CollectConnPoolStats {
		log.Debug("Collecting ConnPoolStats Metrics")
		connPoolStats := commoncollector.GetConnPoolStats(client)
		if connPoolStats != nil {
			connPoolStats.Export(ch)
		}
	}
}

func (exporter *MongodbCollector) collectMongodReplSet(client *mongo.Client, ch chan<- prometheus.Metric) {
	exporter.collectMongod(client, ch)

	log.Debug("Collecting ReplSetConf Metrics")
	replSetConf := mongod.GetReplSetConf(client)
	if replSetConf != nil {
		replSetConf.Export(ch)
	}

	log.Debug("Collecting Replset Status")
	replSetStatus := mongod.GetReplSetStatus(client)
	if replSetStatus != nil {
		replSetStatus.Export(ch)
	}

	log.Debug("Collecting Replset Oplog Status")
	oplogStatus := mongod.GetOplogStatus(client)
	if oplogStatus != nil {
		oplogStatus.Export(ch)
	}
}

// check interface
var _ prometheus.Collector = (*MongodbCollector)(nil)
