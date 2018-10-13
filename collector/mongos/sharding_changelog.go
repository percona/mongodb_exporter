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

package mongos

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	shardingChangelogInfo = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "changelog_10min_total",
		Help:      "Total # of Cluster Balancer log events over the last 10 minutes",
	}, []string{"event"})
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
	shardingChangelogInfo.WithLabelValues("moveChunk.start").Set(0)
	shardingChangelogInfo.WithLabelValues("moveChunk.to").Set(0)
	shardingChangelogInfo.WithLabelValues("moveChunk.to_failed").Set(0)
	shardingChangelogInfo.WithLabelValues("moveChunk.from").Set(0)
	shardingChangelogInfo.WithLabelValues("moveChunk.from_failed").Set(0)
	shardingChangelogInfo.WithLabelValues("moveChunk.commit").Set(0)
	shardingChangelogInfo.WithLabelValues("addShard").Set(0)
	shardingChangelogInfo.WithLabelValues("removeShard.start").Set(0)
	shardingChangelogInfo.WithLabelValues("shardCollection").Set(0)
	shardingChangelogInfo.WithLabelValues("shardCollection.start").Set(0)
	shardingChangelogInfo.WithLabelValues("split").Set(0)
	shardingChangelogInfo.WithLabelValues("multi-split").Set(0)

	// set counts for events found in our query
	for _, item := range *status.Items {
		event := item.Id.Event
		note := item.Id.Note
		count := item.Count
		switch event {
		case "moveChunk.to":
			if note == "success" || note == "" {
				shardingChangelogInfo.WithLabelValues(event).Set(count)
			} else {
				shardingChangelogInfo.WithLabelValues(event + "_failed").Set(count)
			}
		case "moveChunk.from":
			if note == "success" || note == "" {
				shardingChangelogInfo.WithLabelValues(event).Set(count)
			} else {
				shardingChangelogInfo.WithLabelValues(event + "_failed").Set(count)
			}
		default:
			shardingChangelogInfo.WithLabelValues(event).Set(count)
		}
	}
	shardingChangelogInfo.Collect(ch)
}

func (status *ShardingChangelogStats) Describe(ch chan<- *prometheus.Desc) {
	shardingChangelogInfo.Describe(ch)
}

func GetShardingChangelogStatus(session *mgo.Session) *ShardingChangelogStats {
	var qresults []ShardingChangelogSummary
	coll := session.DB("config").C("changelog")
	match := bson.M{"time": bson.M{"$gt": time.Now().Add(-10 * time.Minute)}}
	group := bson.M{"_id": bson.M{"event": "$what", "note": "$details.note"}, "count": bson.M{"$sum": 1}}

	err := coll.Pipe([]bson.M{{"$match": match}, {"$group": group}}).All(&qresults)
	if err != nil {
		log.Error("Failed to execute find query on 'config.changelog'!")
	}

	results := &ShardingChangelogStats{}
	results.Items = &qresults
	return results
}
