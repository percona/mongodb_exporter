package collector_mongos

import (
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	shardingChangelogInfo = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "sharding", "changelog_10min_total"),
		"Total # of Cluster Balancer log events over the last 10 minutes",
		[]string{"event"}, nil)
)

type ShardingChangelogSummaryId struct {
	Event string `bson:"event"`
	Note  string `bson:"note"`
}

type ShardingChangelogSummary struct {
	Id    *ShardingChangelogSummaryId `bson:"_id"`
	Count float64                     `bson:"count"`
}

type ShardingChangelogStats struct {
	Items *[]ShardingChangelogSummary
}

func (status *ShardingChangelogStats) Export(ch chan<- prometheus.Metric) {
	// set all expected event types to zero first, so they show in results if there was no events in the current time period
	outputMetrics := []string{"moveChunk.start", "moveChunk.to", "moveChunk.to_failed", "moveChunk.from", "moveChunk.from_failed", "moveChunk.commit",
		"addShard", "removeShard.start", "shardCollection", "shardCollection.start", "split", "multi-split"}
	counts := make(map[string]float64)
	for _, metricName := range outputMetrics {
		counts[metricName] = 0
	}

	// set counts for events found in our query
	for _, item := range *status.Items {
		event := item.Id.Event
		note := item.Id.Note
		count := item.Count
		switch event {
		case "moveChunk.from", "moveChunk.to":
			if note == "success" || note == "" {
				counts[event] = count
			} else {
				counts[event+"_failed"] = count
			}
		default:
			counts[event] = count
		}
	}

	for metricName, count := range counts {
		ch <- prometheus.MustNewConstMetric(shardingChangelogInfo, prometheus.CounterValue, count, metricName)
	}

}

func (status *ShardingChangelogStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- shardingChangelogInfo
}

func GetShardingChangelogStatus(session *mgo.Session) *ShardingChangelogStats {
	var qresults []ShardingChangelogSummary
	coll := session.DB("config").C("changelog")
	match := bson.M{"time": bson.M{"$gt": time.Now().Add(-10 * time.Minute)}}
	group := bson.M{"_id": bson.M{"event": "$what", "note": "$details.note"}, "count": bson.M{"$sum": 1}}

	err := coll.Pipe([]bson.M{{"$match": match}, {"$group": group}}).All(&qresults)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.changelog'!")
	}

	results := &ShardingChangelogStats{}
	results.Items = &qresults
	return results
}
