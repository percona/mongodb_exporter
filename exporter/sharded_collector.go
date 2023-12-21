// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type shardedCollector struct {
	ctx        context.Context
	base       *baseCollector
	compatible bool
}

// newShardedCollector creates collector collecting metrics about chunks for sharded Mongo.
func newShardedCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, compatibleMode bool) *shardedCollector {
	return &shardedCollector{
		ctx:        ctx,
		base:       newBaseCollector(client, logger),
		compatible: compatibleMode,
	}
}

func (d *shardedCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *shardedCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *shardedCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "sharded")()

	client := d.base.client
	logger := d.base.logger

	databaseNames, err := client.ListDatabaseNames(d.ctx, bson.D{})
	if err != nil {
		logger.Errorf("cannot get database names: %s", err)
	}
	for _, database := range databaseNames {
		collectionNames, err := client.Database(database).ListCollectionNames(d.ctx, bson.D{})
		if err != nil {
			logger.Errorf("cannot get collection names: %s", err)
			continue
		}
		for _, collection := range collectionNames {
			cursor := client.Database(database).Collection(collection)
			if err != nil {
				logger.Errorf("cannot get collection %s:%s", collection, err)
				continue
			}

			rs, err := cursor.Find(d.ctx, bson.M{"_id": bson.M{"$regex": fmt.Sprintf("^%s.", database), "$options": "i"}})
			if err != nil {
				logger.Errorf("cannot find _id starting with \"%s.L\":%s", collection, err)
				continue
			}

			var decoded []bson.M
			err = rs.All(d.ctx, &decoded)
			if err != nil {
				logger.Errorf("cannot get collectionkkk %s:%s", collection, err)
				continue
			}

			for _, row := range decoded {
				if len(row) == 0 {
					continue
				}

				fmt.Println(collection)
				fmt.Println(row)
				fmt.Println("---")

				var chunksMatchPredicate bson.M
				if _, ok := row["timestamp"]; ok {
					if uuid, ok := row["uuid"]; ok {
						chunksMatchPredicate = bson.M{"uuid": uuid}
					}
				} else {
					if id, ok := row["_id"]; ok {
						chunksMatchPredicate = bson.M{"_id": id}
					}
				}

				aggregation := bson.A{
					bson.M{"$match": chunksMatchPredicate},
					bson.M{"$group": bson.M{"_id": "$shard", "cnt": bson.M{"$sum": 1}}},
					bson.M{"$project": bson.M{"_id": 0, "shard": "$_id", "nChunks": "$cnt"}},
					bson.M{"$sort": bson.M{"shard": 1}},
				}

				cur, err := client.Database("config").Collection("chunks").Aggregate(context.Background(), aggregation)
				if err != nil {
					logger.Errorf("cannot get $sharded cursor for collection config.chunks: %s", err)
				}

				var chunks []bson.M
				err = cur.All(context.Background(), &chunks)
				if err != nil {
					logger.Errorf("cannot decode $sharded for collection config.chunks: %s", err)
				}

				for _, c := range chunks {
					if dropped, ok := c["dropped"]; ok && dropped.(bool) {
						continue
					}

					prefix := "sharded collection chunks"
					labels := make(map[string]string)
					labels["database"] = database
					labels["collection"] = collection
					labels["shard"] = c["shard"].(string)

					logger.Debug("$sharded metrics for config.chunks")
					debugResult(logger, chunks)

					m := primitive.M{"count": c["nChunks"].(int32)}
					for _, metric := range makeMetrics(prefix, m, labels, d.compatible) {
						fmt.Println(metric.Desc().String())
						ch <- metric
					}
				}
			}
		}
	}
}

var _ prometheus.Collector = (*shardedCollector)(nil)
