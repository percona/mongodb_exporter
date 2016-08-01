package collector

import (
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

type OplogTimestamps struct {
	Tail	float64
	Head	float64
}

type OplogStatus struct {
	OplogTimestamps	*OplogTimestamps
	CollectionStats	*OplogCollectionStats
}

// there's gotta be a better way to do this, but it works for now :/
func BsonMongoTimestampToUnix(timestamp bson.MongoTimestamp) float64 {
	return float64(timestamp >> 32)
}

func GetOplogTimestamps(session *mgo.Session) (*OplogTimestamps, error) {
	oplogTimestamps := &OplogTimestamps{}
	var err error

	// retry once if there is an error
	var tries int64 = 0
	var head_result struct { Timestamp	bson.MongoTimestamp	`bson:"ts"` }
	for tries < 2 {
		err = session.DB("local").C("oplog.rs").Find(nil).Sort("-$natural").Limit(1).One(&head_result)
		if err == nil {
			break
		}
		tries += 1
	}
	if err != nil {
		return oplogTimestamps, err
	}

	// retry once if there is an error
	tries = 0
	var tail_result struct { Timestamp	bson.MongoTimestamp	`bson:"ts"` }
	for tries < 2 {
		err = session.DB("local").C("oplog.rs").Find(nil).Sort("$natural").Limit(1).One(&tail_result)
		if err == nil {
			break
		}
		tries += 1
	}
	if err != nil {
		return oplogTimestamps, err
	}

	oplogTimestamps.Tail = BsonMongoTimestampToUnix(tail_result.Timestamp)
	oplogTimestamps.Head = BsonMongoTimestampToUnix(head_result.Timestamp)
	return oplogTimestamps, err
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
	if status.OplogTimestamps != nil {
		oplogStatusHeadTimestamp.Set(status.OplogTimestamps.Head)
		oplogStatusTailTimestamp.Set(status.OplogTimestamps.Tail)
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
	collectionStats, err := GetOplogCollectionStats(session)
	oplogTimestamps, err := GetOplogTimestamps(session)
	if err != nil {
		glog.Error("Failed to get oplog status.")
		return nil
	}

	return &OplogStatus{CollectionStats:collectionStats,OplogTimestamps:oplogTimestamps}
}
