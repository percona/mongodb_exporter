package collector_mongod

import (
	"time"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	oplogStatusCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"replset_oplog",
		Name:		"items_total",
		Help:		"The total number of changes in the oplog",
	})
	oplogStatusHeadTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"replset_oplog",
		Name:		"head_timestamp",
		Help:		"The timestamp of the newest change in the oplog",
	})
	oplogStatusTailTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"replset_oplog",
		Name:		"tail_timestamp",
		Help:		"The timestamp of the oldest change in the oplog",
	})
	oplogStatusSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"replset_oplog",
		Name:		"size_bytes",
		Help:		"Size of oplog in bytes",
	}, []string{"type"})
)

type OplogCollectionStats struct {
	Count		float64	`bson:"count"`
	Size		float64	`bson:"size"`
	StorageSize	float64 `bson:"storageSize"`
}

type OplogStatus struct {
	TailTimestamp	float64
	HeadTimestamp	float64
	CollectionStats	*OplogCollectionStats
}

// there's gotta be a better way to do this, but it works for now :/
func BsonMongoTimestampToUnix(timestamp bson.MongoTimestamp) float64 {
	return float64(timestamp >> 32)
}

func GetOplogTimestamp(session *mgo.Session, returnTail bool) (float64, error) {
	var sortBy string = "$natural"
	if returnTail {
		sortBy = "-$natural"
	}

	var err error
	var tries int    = 0
	var maxTries int = 2
	var result struct { Timestamp bson.MongoTimestamp `bson:"ts"` }
	for tries < maxTries {
		err = session.DB("local").C("oplog.rs").Find(nil).Sort(sortBy).Limit(1).One(&result)
		if err != nil {
			tries += 1
			time.Sleep(500 * time.Millisecond)
		} else {
			return BsonMongoTimestampToUnix(result.Timestamp), err
		}
	}

	return 0, err
}

func GetOplogCollectionStats(session *mgo.Session) (*OplogCollectionStats, error) {
	results := &OplogCollectionStats{}
	err := session.DB("local").Run(bson.M{ "collStats" : "oplog.rs" }, &results)
	return results, err
}

func (status *OplogStatus) Export(ch chan<- prometheus.Metric) {
	oplogStatusSizeBytes.WithLabelValues("current").Set(0)
	oplogStatusSizeBytes.WithLabelValues("storage").Set(0)
	if status.CollectionStats != nil {
		oplogStatusCount.Set(status.CollectionStats.Count)
		oplogStatusSizeBytes.WithLabelValues("current").Set(status.CollectionStats.Size)
		oplogStatusSizeBytes.WithLabelValues("storage").Set(status.CollectionStats.StorageSize)
	}
	if status.HeadTimestamp != 0 && status.TailTimestamp != 0 {
		oplogStatusHeadTimestamp.Set(status.HeadTimestamp)
		oplogStatusTailTimestamp.Set(status.TailTimestamp)
	}

	oplogStatusCount.Collect(ch)
	oplogStatusHeadTimestamp.Collect(ch)
	oplogStatusTailTimestamp.Collect(ch)
	oplogStatusSizeBytes.Collect(ch)
}

func (status *OplogStatus) Describe(ch chan<- *prometheus.Desc) {
	oplogStatusCount.Describe(ch)
	oplogStatusHeadTimestamp.Describe(ch)
	oplogStatusTailTimestamp.Describe(ch)
	oplogStatusSizeBytes.Describe(ch)
}

func GetOplogStatus(session *mgo.Session) *OplogStatus {
	oplogStatus          := &OplogStatus{}
	collectionStats, err := GetOplogCollectionStats(session)
	if err != nil {
		glog.Error("Failed to get local.oplog_rs collection stats.")
		return nil
	}

	headTimestamp, err := GetOplogTimestamp(session, false)
	tailTimestamp, err := GetOplogTimestamp(session, true)
	if err != nil {
		glog.Error("Failed to get oplog head or tail timestamps.")
		return nil
	}

	oplogStatus.CollectionStats = collectionStats
	oplogStatus.HeadTimestamp   = headTimestamp
	oplogStatus.TailTimestamp   = tailTimestamp

	return oplogStatus
}
