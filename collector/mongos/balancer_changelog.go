package collector_mongos

import (
    "time"
    "github.com/golang/glog"
    "github.com/prometheus/client_golang/prometheus"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
)

var (
    balancerChangelogInfo = prometheus.NewCounterVec(prometheus.CounterOpts{
            Namespace: Namespace,
            Name:      "balancer_changelog",
            Help:      "Log event statistics for the MongoDB balancer",
    }, []string{"event"})
)

type BalancerChangelogAggregationId struct {
    Event	string	`bson:"event"`
    Note	string	`bson:"note"`
}

type BalancerChangelogAggregationResult struct {
    Id		*BalancerChangelogAggregationId	`bson:"_id"`
    Count 	float64				`bson:"count"`
}

type BalancerChangelogStats struct {
    MoveChunkStart              float64
    MoveChunkFromSuccess        float64
    MoveChunkFromFailed         float64
    MoveChunkToSuccess          float64
    MoveChunkToFailed           float64
    MoveChunkCommit             float64
    Split                       float64
    MultiSplit                  float64
    ShardCollection             float64
    ShardCollectionStart        float64
    AddShard                    float64
}

func GetBalancerChangelogStats24hr(session *mgo.Session, showErrors bool) *BalancerChangelogStats {
    var qresults []BalancerChangelogAggregationResult
    coll  := session.DB("config").C("changelog")
    match := bson.M{ "time" : bson.M{ "$gt" : time.Now().Add(-24 * time.Hour) } }
    group := bson.M{ "_id" : bson.M{ "event" : "$what", "note" : "$details.note" }, "count" : bson.M{ "$sum" : 1 } }

    err := coll.Pipe([]bson.M{ { "$match" : match }, { "$group" : group } }).All(&qresults)
    if err != nil {
        glog.Error("Error executing aggregation on 'config.changelog'!")
    }

    results := &BalancerChangelogStats{}
    for _, stat := range qresults {
        event := stat.Id.Event
        note  := stat.Id.Note
        count := stat.Count
        if event == "moveChunk.start" {
            results.MoveChunkStart = count
        } else if event == "moveChunk.to" {
            if note == "success" {
                results.MoveChunkToSuccess = count
            } else {
                results.MoveChunkToFailed = count
            }
        } else if event == "moveChunk.from" {
            if note == "success" {
                results.MoveChunkFromSuccess = count
            } else {
                results.MoveChunkFromFailed = count
            }
        } else if event == "moveChunk.commit" {
            results.MoveChunkCommit = count
        } else if event == "addShard" {
            results.AddShard = count
        } else if event == "shardCollection" {
            results.ShardCollection = count
        } else if event == "shardCollection.start" {
            results.ShardCollectionStart = count
        } else if event == "split" {
            results.Split = count
        } else if event == "multi-split" {
            results.MultiSplit = count
        }
    }

    return results
}

func (status *BalancerChangelogStats) Export(ch chan<- prometheus.Metric) {
    balancerChangelogInfo.WithLabelValues("moveChunk.start").Set(status.MoveChunkStart)
    balancerChangelogInfo.WithLabelValues("moveChunk.to").Set(status.MoveChunkToSuccess)
    balancerChangelogInfo.WithLabelValues("moveChunk.to_failed").Set(status.MoveChunkToFailed)
    balancerChangelogInfo.WithLabelValues("moveChunk.from").Set(status.MoveChunkFromSuccess)
    balancerChangelogInfo.WithLabelValues("moveChunk.from_failed").Set(status.MoveChunkFromFailed)
    balancerChangelogInfo.WithLabelValues("moveChunk.commit").Set(status.MoveChunkCommit)
    balancerChangelogInfo.WithLabelValues("addShard").Set(status.AddShard)
    balancerChangelogInfo.WithLabelValues("shardCollection").Set(status.ShardCollection)
    balancerChangelogInfo.WithLabelValues("shardCollection.start").Set(status.ShardCollectionStart)
    balancerChangelogInfo.WithLabelValues("split").Set(status.Split)
    balancerChangelogInfo.WithLabelValues("multi-split").Set(status.MultiSplit)
    balancerChangelogInfo.Collect(ch)
}

func GetBalancerChangelogStatus(session *mgo.Session) *BalancerChangelogStats {
    session.SetMode(mgo.Eventual, true)
    session.SetSocketTimeout(0)
    results := GetBalancerChangelogStats24hr(session, false)
    return results
}
