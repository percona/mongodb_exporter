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
	"log/slog"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type shardsCollector struct {
	ctx        context.Context
	base       *baseCollector
	compatible bool
}

// newShardsCollector creates collector collecting metrics about chunks for shards Mongo.
func newShardsCollector(ctx context.Context, client *mongo.Client, logger *slog.Logger, compatibleMode bool) *shardsCollector {
	return &shardsCollector{
		ctx:        ctx,
		base:       newBaseCollector(client, logger.With("collector", "shards")),
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
	ctx := d.ctx

	metrics := make([]prometheus.Metric, 0)
	metric, err := chunksTotal(ctx, client)
	if err != nil {
		logger.Warn("cannot create metric for chunks total", "error", err)
	} else {
		metrics = append(metrics, metric)
	}

	ms, err := chunksTotalPerShard(ctx, client)
	if err != nil {
		logger.Warn("cannot create metric for chunks total per shard", "error", err)
	} else {
		metrics = append(metrics, ms...)
	}

	for _, metric := range metrics {
		ch <- metric
	}

	databaseNames, err := client.ListDatabaseNames(d.ctx, bson.D{})
	if err != nil {
		logger.Error("cannot get database names", "error", err)
	}
	for _, database := range databaseNames {
		collections := d.getCollectionsForDBName(database)
		for _, row := range collections {
			if len(row) == 0 {
				continue
			}

			var ok bool
			if _, ok = row[idKey]; !ok {
				continue
			}
			var rowID string
			if rowID, ok = row[idKey].(string); !ok {
				continue
			}

			chunks := d.getChunksForCollection(row)
			for _, c := range chunks {
				labels, chunks, success := d.getInfoForChunk(c, database, rowID)
				if !success {
					continue
				}
				for _, metric := range makeMetrics(prefix, bson.M{countKey: chunks}, labels, d.compatible) {
					ch <- metric
				}
			}
		}
	}
}

func (d *shardsCollector) getInfoForChunk(c bson.M, database, rowID string) (map[string]string, int32, bool) {
	var ok bool
	if _, ok = c["dropped"]; ok {
		if dropped, ok := c["dropped"].(bool); ok && dropped {
			return nil, 0, false
		}
	}

	if _, ok = c[shardKey]; !ok {
		return nil, 0, ok
	}
	var shard string
	if shard, ok = c[shardKey].(string); !ok {
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
	labels[shardKey] = shard

	logger := d.base.logger
	logger.Debug("$shards metrics for config.chunks")
	debugResult(d.ctx, logger, bson.M{database: c})

	return labels, chunks, true
}

func (d *shardsCollector) getCollectionsForDBName(database string) []bson.M {
	client := d.base.client
	logger := d.base.logger

	cursor := client.Database("config").Collection("collections")
	rs, err := cursor.Find(d.ctx, bson.M{idKey: bson.M{"$regex": fmt.Sprintf("^%s.", database), "$options": "i"}})
	if err != nil {
		logger.Error("cannot find _id with database prefix", "database", database, "error", err)
		return nil
	}

	var decoded []bson.M
	err = rs.All(d.ctx, &decoded)
	if err != nil {
		logger.Error("cannot decode collections", "error", err)
		return nil
	}

	return decoded
}

func (d *shardsCollector) getChunksForCollection(row bson.M) []bson.M {
	var chunksMatchPredicate bson.M
	if _, ok := row["timestamp"]; ok {
		if uuid, ok := row["uuid"]; ok {
			chunksMatchPredicate = bson.M{"uuid": uuid}
		}
	} else {
		if id, ok := row[idKey]; ok {
			chunksMatchPredicate = bson.M{idKey: id}
		}
	}

	aggregation := bson.A{
		bson.M{"$match": chunksMatchPredicate},
		bson.M{groupOperator: bson.M{idKey: "$shard", "cnt": bson.M{sumOperator: 1}}},
		bson.M{"$project": bson.M{idKey: 0, shardKey: idReference, "nChunks": "$cnt"}},
		bson.M{"$sort": bson.M{shardKey: 1}},
	}

	client := d.base.client
	logger := d.base.logger

	cur, err := client.Database("config").Collection("chunks").Aggregate(context.Background(), aggregation)
	if err != nil {
		logger.Error("cannot get $shards cursor for collection config.chunks", "error", err)
		return nil
	}

	var chunks []bson.M
	err = cur.All(context.Background(), &chunks)
	if err != nil {
		logger.Error("cannot decode $shards for collection config.chunks", "error", err)
		return nil
	}

	return chunks
}

func chunksTotal(ctx context.Context, client *mongo.Client) (prometheus.Metric, error) { //nolint:ireturn
	n, err := client.Database("config").Collection("chunks").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot get total number of chunks")
	}

	name := "mongodb_mongos_sharding_chunks_total"
	help := "Total number of chunks"

	d := prometheus.NewDesc(name, help, nil, nil)
	return prometheus.NewConstMetric(d, prometheus.GaugeValue, float64(n))
}

func chunksTotalPerShard(ctx context.Context, client *mongo.Client) ([]prometheus.Metric, error) {
	aggregation := bson.D{
		{Key: groupOperator, Value: bson.M{idKey: "$shard", countKey: bson.M{sumOperator: 1}}},
	}

	cursor, err := client.Database("config").Collection("chunks").Aggregate(ctx, mongo.Pipeline{aggregation})
	if err != nil {
		return nil, errors.Wrap(err, "cannot get $shards cursor for collection config.chunks")
	}

	var shards []bson.M
	if err = cursor.All(ctx, &shards); err != nil {
		return nil, errors.Wrap(err, "cannot get $shards for collection config.chunks")
	}

	metrics := make([]prometheus.Metric, 0, len(shards))

	for _, shard := range shards {
		help := "Total number of chunks per shard"
		id, ok := shard[idKey].(string)
		if !ok {
			continue
		}
		labels := map[string]string{shardKey: id}

		d := prometheus.NewDesc("mongodb_mongos_sharding_shard_chunks_total", help, nil, labels)
		val, ok := shard[countKey].(int32)
		if !ok {
			continue
		}

		metric, err := prometheus.NewConstMetric(d, prometheus.GaugeValue, float64(val))
		if err != nil {
			continue
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

var _ prometheus.Collector = (*shardsCollector)(nil)
