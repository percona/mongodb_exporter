package collector_mongod

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
		Name:		"current_items",
		Help:		"The current number of items in the oplog",
	})
	oplogStatusHeadTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"replset_oplog",
		Name:		"head_timestamp",
		Help:		"The unix timestamp of the newest change in the oplog",
	})
	oplogStatusTailTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"replset_oplog",
		Name:		"tail_timestamp",
		Help:		"The unix timestamp of the oldest change in the oplog",
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
	result := struct {
		TailTimestamp	bson.MongoTimestamp	`bson:"tail"`
		HeadTimestamp	bson.MongoTimestamp	`bson:"head"`
	}{}
	group := bson.M{ "_id" : 1, "tail" : bson.M{ "$min" : "$ts" }, "head" : bson.M{ "$max" : "$ts" } }
	err := session.DB("local").C("oplog.rs").Pipe([]bson.M{{ "$group" : group  }}).One(&result)
	if err != nil {
		return oplogTimestamps, err
	}

	oplogTimestamps.Tail = BsonMongoTimestampToUnix(result.TailTimestamp)
	oplogTimestamps.Head = BsonMongoTimestampToUnix(result.HeadTimestamp)
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
