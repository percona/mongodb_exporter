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
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type shardsCollector struct {
	ctx        context.Context
	base       *baseCollector
	compatible bool
}

// newShardsCollector creates collector collecting metrics about chunks for shards Mongo.
func newShardsCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, compatibleMode bool) *shardsCollector {
	return &shardsCollector{
		ctx:        ctx,
		base:       newBaseCollector(client, logger),
		compatible: compatibleMode,
	}
}

func (d *shardsCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *shardsCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *shardsCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "shards")()

	client := d.base.client
	logger := d.base.logger
	prefix := "shards collection chunks"

	databaseNames, err := client.ListDatabaseNames(d.ctx, bson.D{})
	if err != nil {
		logger.Errorf("cannot get database names: %s", err)
	}
	for _, database := range databaseNames {
		collections := d.getCollectionsForDBName(database)
		for _, row := range collections {
			if len(row) == 0 {
				continue
			}

			var ok bool
			if _, ok = row["_id"]; !ok {
				continue
			}
			var rowID string
			if rowID, ok = row["_id"].(string); !ok {
				continue
			}

			chunks := d.getChunksForCollection(row)
			for _, c := range chunks {
				labels, chunks, success := d.getInfoForChunk(c, database, rowID)
				if !success {
					continue
				}
				for _, metric := range makeMetrics(prefix, primitive.M{"count": chunks}, labels, d.compatible) {
					ch <- metric
				}
			}
		}
	}
}

func (d *shardsCollector) getInfoForChunk(c primitive.M, database, rowID string) (map[string]string, int32, bool) {
	var ok bool
	if _, ok = c["dropped"]; ok {
		if dropped, ok := c["dropped"].(bool); ok && dropped {
			return nil, 0, false
		}
	}

	if _, ok = c["shard"]; !ok {
		return nil, 0, ok
	}
	var shard string
	if shard, ok = c["shard"].(string); !ok {
		return nil, 0, ok
	}

	if _, ok = c["nChunks"]; !ok {
		return nil, 0, ok
	}
	var chunks int32
	if chunks, ok = c["nChunks"].(int32); !ok {
		return nil, 0, ok
	}

	labels := make(map[string]string)
	labels["database"] = database
	labels["collection"] = strings.Replace(rowID, fmt.Sprintf("%s.", database), "", 1)
	labels["shard"] = shard

	logger := d.base.logger
	logger.Debug("$shards metrics for config.chunks")
	debugResult(logger, primitive.M{database: c})

	return labels, chunks, true
}

func (d *shardsCollector) getCollectionsForDBName(database string) []primitive.M {
	client := d.base.client
	logger := d.base.logger

	cursor := client.Database("config").Collection("collections")
	rs, err := cursor.Find(d.ctx, bson.M{"_id": bson.M{"$regex": fmt.Sprintf("^%s.", database), "$options": "i"}})
	if err != nil {
		logger.Errorf("cannot find _id starting with \"%s.\":%s", database, err)
		return nil
	}

	var decoded []bson.M
	err = rs.All(d.ctx, &decoded)
	if err != nil {
		logger.Errorf("cannot decode collections: %s", err)
		return nil
	}

	return decoded
}

func (d *shardsCollector) getChunksForCollection(row primitive.M) []bson.M {
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

	client := d.base.client
	logger := d.base.logger

	cur, err := client.Database("config").Collection("chunks").Aggregate(context.Background(), aggregation)
	if err != nil {
		logger.Errorf("cannot get $shards cursor for collection config.chunks: %s", err)
		return nil
	}

	var chunks []bson.M
	err = cur.All(context.Background(), &chunks)
	if err != nil {
		logger.Errorf("cannot decode $shards for collection config.chunks: %s", err)
		return nil
	}

	return chunks
}

var _ prometheus.Collector = (*shardsCollector)(nil)
