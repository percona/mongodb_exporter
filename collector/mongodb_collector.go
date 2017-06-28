package collector

import (
	"github.com/percona/mongodb_exporter/collector/mongod"
	"github.com/percona/mongodb_exporter/collector/mongos"
	"github.com/percona/mongodb_exporter/shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
)

var (
	// Namespace is the namespace of the metrics
	Namespace = "mongodb"
)

// MongodbCollectorOpts is the options of the mongodb collector.
type MongodbCollectorOpts struct {
	URI                   string
	TLSConnection         bool
	TLSCertificateFile    string
	TLSPrivateKeyFile     string
	TLSCaFile             string
	TLSHostnameValidation bool
}

func (in MongodbCollectorOpts) toSessionOps() shared.MongoSessionOpts {
	return shared.MongoSessionOpts{
		URI:                   in.URI,
		TLSConnection:         in.TLSConnection,
		TLSCertificateFile:    in.TLSCertificateFile,
		TLSPrivateKeyFile:     in.TLSPrivateKeyFile,
		TLSCaFile:             in.TLSCaFile,
		TLSHostnameValidation: in.TLSHostnameValidation,
	}
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
	log.Debug("Describing groups")
	session := shared.MongoSession(exporter.Opts.toSessionOps())
	if session != nil {
		serverStatus := collector_mongos.GetServerStatus(session)
		if serverStatus != nil {
			serverStatus.Describe(ch)
		}
		session.Close()
	}
}

// Collect collects all mongodb's metrics.
func (exporter *MongodbCollector) Collect(ch chan<- prometheus.Metric) {
	mongoSess := shared.MongoSession(exporter.Opts.toSessionOps())
	if mongoSess != nil {
		defer mongoSess.Close()
		serverVersion, err := shared.MongoSessionServerVersion(mongoSess)
		if err != nil {
			log.Errorf("Problem gathering the mongo server version: %s", err)
		}

		nodeType, err := shared.MongoSessionNodeType(mongoSess)
		if err != nil {
			log.Errorf("Problem gathering the mongo node type: %s", err)
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
			log.Errorf("Unrecognized node type %s!", nodeType)
		}
	}
}

func (exporter *MongodbCollector) collectMongos(session *mgo.Session, ch chan<- prometheus.Metric) {
	// read from primaries only when using mongos to avoid SERVER-27864
	session.SetMode(mgo.Strong, true)

	log.Debug("Collecting Server Status")
	serverStatus := collector_mongos.GetServerStatus(session)
	if serverStatus != nil {
		serverStatus.Export(ch)
	}

	log.Debug("Collecting Sharding Status")
	shardingStatus := collector_mongos.GetShardingStatus(session)
	if shardingStatus != nil {
		shardingStatus.Export(ch)
	}
}

func (exporter *MongodbCollector) collectMongod(session *mgo.Session, ch chan<- prometheus.Metric) {
	log.Debug("Collecting Server Status")
	serverStatus := collector_mongod.GetServerStatus(session)
	if serverStatus != nil {
		serverStatus.Export(ch)
	}
}

func (exporter *MongodbCollector) collectMongodReplSet(session *mgo.Session, ch chan<- prometheus.Metric) {
	exporter.collectMongod(session, ch)

	log.Debug("Collecting Replset Status")
	replSetStatus := collector_mongod.GetReplSetStatus(session)
	if replSetStatus != nil {
		replSetStatus.Export(ch)
	}

	log.Debug("Collecting Replset Oplog Status")
	oplogStatus := collector_mongod.GetOplogStatus(session)
	if oplogStatus != nil {
		oplogStatus.Export(ch)
	}
}
