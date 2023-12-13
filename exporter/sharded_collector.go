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
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type shardedCollector struct {
	ctx          context.Context
	base         *baseCollector
	topologyInfo labelsGetter
}

// newShardedCollector creates collector collecting metrics about chunks for sharded Mongo.
func newShardedCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, topology labelsGetter) *shardedCollector {
	return &shardedCollector{
		ctx:          ctx,
		base:         newBaseCollector(client, logger),
		topologyInfo: topology,
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

	aggregation := bson.D{
		{Key: "$group", Value: bson.M{"_id": "$shard", "count": bson.M{"$sum": 1}, "ns": bson.M{"$first": "$ns"}}},
	}
	cur, err := client.Database("config").Collection("chunks").Aggregate(context.Background(), mongo.Pipeline{aggregation})
	if err != nil {
		logger.Errorf("cannot get $sharded cursor for collection config.chunks: %s", err)
	}

	var chunks []bson.M
	err = cur.All(context.Background(), &chunks)
	if err != nil {
		logger.Errorf("cannot get $sharded for collection config.chunks: %s", err)
	}

	logger.Debug("$sharded metrics for config.chunks")
	debugResult(logger, chunks)

	for _, c := range chunks {
		namespace := c["ns"].(string)
		split := strings.SplitN(namespace, ".", 1)
		database := split[0]
		collection := ""
		if len(split) >= 2 {
			collection = split[1]
		}
		prefix := "sharded"
		labels := d.topologyInfo.baseLabels()
		labels["database"] = database
		labels["collection"] = collection

		for _, metric := range makeMetrics(prefix, c, labels, true) {
			ch <- metric
		}
	}
}

var _ prometheus.Collector = (*shardedCollector)(nil)