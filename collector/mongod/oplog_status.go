package collector_mongod

import (
    "time"
    "github.com/golang/glog"
    "github.com/prometheus/client_golang/prometheus"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
)

var (
    oplogStatusLengthSec = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "oplog",
            Name:      "length_sec",
            Help:      "Length of oplog in seconds from head to tail",
    })
    oplogStatusLengthSecNow = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "oplog",
            Name:      "length_sec_now",
            Help:      "Length of oplog in seconds from now to tail",
    })
    oplogStatusSizeMB = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "oplog",
            Name:      "size_mb",
            Help:      "Size of oplog in megabytes",
    })
)

func GetCollectionSizeMB(db string, collection string, session *mgo.Session) (float64) {
    var collStats map[string]interface{}
    err := session.DB(db).Run(bson.D{{"collStats", collection }}, &collStats)
    if err != nil {
        glog.Error("Error getting collection stats!")
    }

    var result float64 = -1
    if collStats["size"] != nil {
        size := collStats["size"].(int)
        result = float64(size)/1024/1024
    }

    return result
}

func GetOplogSizeMB(session *mgo.Session) (float64) {
    return GetCollectionSizeMB("local", "oplog.rs", session)
}

func ParseBsonMongoTsToUnix(timestamp bson.MongoTimestamp) (int64) {
    return int64(timestamp >> 32)
}

type OplogStatsData struct {
    MinTime	bson.MongoTimestamp	`bson:"min"`
    MaxTime	bson.MongoTimestamp	`bson:"max"`
}

func GetOplogLengthSecs(session *mgo.Session) (float64, float64) {
    results := &OplogStatsData{}
    group := bson.M{ "_id" : 1, "min" : bson.M{ "$min" : "$ts" }, "max" : bson.M{ "$max" : "$ts" } }
    err := session.DB("local").C("oplog.rs").Pipe([]bson.M{{ "$group" : group  }}).One(&results)
    if err != nil {
        glog.Error("Could not get the oplog time min/max!")
        return -1, -1
    }

    minTime := ParseBsonMongoTsToUnix(results.MinTime)
    maxTime := ParseBsonMongoTsToUnix(results.MaxTime)

    now := time.Now().Unix()
    lengthSeconds := maxTime - minTime
    lengthSecondsNow := now - minTime

    return float64(lengthSeconds), float64(lengthSecondsNow)
}

type OplogStats struct {
    LengthSec		float64
    LengthSecNow	float64
    SizeMB		float64
}

func (status *OplogStats) Export(ch chan<- prometheus.Metric) {
    oplogStatusLengthSec.Set(status.LengthSec)
    oplogStatusLengthSecNow.Set(status.LengthSecNow)
    oplogStatusSizeMB.Set(status.SizeMB)
    oplogStatusLengthSec.Collect(ch)
    oplogStatusLengthSecNow.Collect(ch)
    oplogStatusSizeMB.Collect(ch)
}

func GetOplogStatus(session *mgo.Session) *OplogStats {
    results := &OplogStats{}

    results.LengthSec, results.LengthSecNow = GetOplogLengthSecs(session)
    results.SizeMB = GetOplogSizeMB(session)

    return results
}
