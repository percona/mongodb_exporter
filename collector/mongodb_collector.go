package collector

import (
	"github.com/percona/mongodb_exporter/shared"
	"github.com/percona/mongodb_exporter/collector/mongod"
	"github.com/percona/mongodb_exporter/collector/mongos"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/mgo.v2"
)

var (
	// Namespace is the namespace of the metrics
	Namespace = "mongodb"
)

// MongodbCollectorOpts is the options of the mongodb collector.
type MongodbCollectorOpts struct {
	URI string
}

// MongodbCollector is in charge of collecting mongodb's metrics.
type MongodbCollector struct {
	Opts MongodbCollectorOpts
}

// NewMongodbCollector returns a new instance of a MongodbCollector.
func NewMongodbCollector(opts MongodbCollectorOpts) *MongodbCollector {
	exporter := &MongodbCollector{
		Opts: opts,
	}

	return exporter
}

// Describe describes all mongodb's metrics.
func (exporter *MongodbCollector) Describe(ch chan<- *prometheus.Desc) {
	glog.Info("Describing groups")
	session := shared.MongoSession(exporter.Opts.URI)
	defer session.Close()
	if session != nil {
		serverStatus := collector_mongos.GetServerStatus(session)
		if serverStatus != nil {
			serverStatus.Describe(ch)
		}
	}
}

// Collect collects all mongodb's metrics.
func (exporter *MongodbCollector) Collect(ch chan<- prometheus.Metric) {
	mongoSess := shared.MongoSession(exporter.Opts.URI)
	defer mongoSess.Close()
	if mongoSess != nil {
		serverVersion, err := shared.MongoSessionServerVersion(mongoSess)
		if err != nil {
			glog.Errorf("Problem gathering the mongo server version: %s", err)
		}

		nodeType, err := shared.MongoSessionNodeType(mongoSess)
		if err != nil {
			glog.Errorf("Problem gathering the mongo node type: %s", err)
		}

		glog.Infof("Connected to: %s (node type: %s, server version: %s)", exporter.Opts.URI, nodeType, serverVersion)
		switch {
			case nodeType == "mongos":
				exporter.collectMongos(mongoSess, ch)
			case nodeType == "mongod":
				exporter.collectMongod(mongoSess, ch)
			case nodeType == "replset":
				exporter.collectMongodReplSet(mongoSess, ch)
			default:
				glog.Infof("Unrecognized node type %s!", nodeType)
		}
	}
}

func (exporter *MongodbCollector) collectMongos(session *mgo.Session, ch chan<- prometheus.Metric) {
	glog.Info("Collecting Server Status")
	serverStatus := collector_mongos.GetServerStatus(session)
	if serverStatus != nil {
		serverStatus.Export(ch)
	}

	glog.Info("Collecting Sharding Status")
	shardingStatus := collector_mongos.GetShardingStatus(session)
	if shardingStatus != nil {
		shardingStatus.Export(ch)
	}
}

func (exporter *MongodbCollector) collectMongod(session *mgo.Session, ch chan<- prometheus.Metric) {
	glog.Info("Collecting Server Status")
	serverStatus := collector_mongod.GetServerStatus(session)
	if serverStatus != nil {
		serverStatus.Export(ch)
	}
}

func (exporter *MongodbCollector) collectMongodReplSet(session *mgo.Session, ch chan<- prometheus.Metric) {
	exporter.collectMongod(session, ch)

	glog.Info("Collecting Replset Status")
	replSetStatus := collector_mongod.GetReplSetStatus(session)
	if replSetStatus != nil {
		replSetStatus.Export(ch)
	}       

	glog.Info("Collecting Replset Oplog Status")
	oplogStatus := collector_mongod.GetOplogStatus(session)
	if oplogStatus != nil {
		oplogStatus.Export(ch)
	}       
}

