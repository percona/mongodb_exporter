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
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	shardingChangelogInfoDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "sharding", "changelog_10min_total"),
		"Total # of Cluster Balancer log events over the last 10 minutes",
		[]string{"event"},
		nil,
	)
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
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "moveChunk.start")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "moveChunk.to")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "moveChunk.to_failed")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "moveChunk.from")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "moveChunk.from_failed")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "moveChunk.commit")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "addShard")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "removeShard.start")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "shardCollection")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "shardCollection.start")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "split")
	ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, 0, "multi-split")

	// set counts for events found in our query
	for _, item := range *status.Items {
		event := item.Id.Event
		note := item.Id.Note
		count := item.Count
		switch event {
		case "moveChunk.to":
			if note == "success" || note == "" {
				ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, count, event)

			} else {
				ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, count, event+"_failed")

			}
		case "moveChunk.from":
			if note == "success" || note == "" {
				ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, count, event)

			} else {
				ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, count, event+"_failed")

			}
		default:
			ch <- prometheus.MustNewConstMetric(shardingChangelogInfoDesc, prometheus.CounterValue, count, event)

		}
	}
}

func (status *ShardingChangelogStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- shardingChangelogInfoDesc
}

// GetShardingChangelogStatus gets sharding changelog status.
func GetShardingChangelogStatus(client *mongo.Client) *ShardingChangelogStats {
	var qresults []ShardingChangelogSummary
	coll := client.Database("config").Collection("changelog")
	match := bson.M{"time": bson.M{"$gt": time.Now().Add(-10 * time.Minute)}}
	group := bson.M{"_id": bson.M{"event": "$what", "note": "$details.note"}, "count": bson.M{"$sum": 1}}

	c, err := coll.Aggregate(context.TODO(), []bson.M{{"$match": match}, {"$group": group}})
	if err != nil {
		log.Errorf("Failed to aggregate sharding changelog events: %s.", err)
	}

	defer c.Close(context.TODO())

	for c.Next(context.TODO()) {
		s := &ShardingChangelogSummary{}
		if err := c.Decode(s); err != nil {
			log.Error(err)
			continue
		}
		qresults = append(qresults, *s)
	}

	if err := c.Err(); err != nil {
		log.Error(err)
	}

	results := &ShardingChangelogStats{}
	results.Items = &qresults
	return results
}
