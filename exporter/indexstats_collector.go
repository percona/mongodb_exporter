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

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type indexstatsCollector struct {
	ctx  context.Context
	base *baseCollector

	discoveringMode         bool
	overrideDescendingIndex bool
	topologyInfo            labelsGetter

	collections []string
}

// newIndexStatsCollector creates a collector for statistics on index usage.
func newIndexStatsCollector(ctx context.Context, client *mongo.Client, logger *slog.Logger, discovery, overrideDescendingIndex bool, topology labelsGetter, collections []string) *indexstatsCollector {
	return &indexstatsCollector{
		ctx:  ctx,
		base: newBaseCollector(client, logger.With("collector", "indexstats")),

		discoveringMode:         discovery,
		topologyInfo:            topology,
		overrideDescendingIndex: overrideDescendingIndex,

		collections: collections,
	}
}

func (d *indexstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *indexstatsCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *indexstatsCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "indexstats")()

	client := d.base.client
	logger := d.base.logger

	var collections []string
	if d.discoveringMode {
		onlyCollectionsNamespaces, err := listAllCollections(d.ctx, client, d.collections, systemDBs, true)
		if err != nil {
			logger.Error("cannot auto discover databases and collections", "error", err.Error())

			return
		}

		collections = fromMapToSlice(onlyCollectionsNamespaces)
	} else {
		var err error
		collections, err = checkNamespacesForViews(d.ctx, client, d.collections)
		if err != nil {
			logger.Error("cannot list collections", "error", err.Error())

			return
		}
	}

	for _, dbCollection := range collections {
		parts := strings.Split(dbCollection, ".")
		if len(parts) < 2 { //nolint:gomnd
			continue
		}

		database := parts[0]
		collection := strings.Join(parts[1:], ".")

		// exclude system collections
		if strings.HasPrefix(collection, "system.") {
			continue
		}

		aggregation := bson.D{
			{Key: "$indexStats", Value: bson.M{}},
		}

		cursor, err := client.Database(database).Collection(collection).Aggregate(d.ctx, mongo.Pipeline{aggregation})
		if err != nil {
			logger.Error("cannot get $indexStats cursor for collection", "database", database, "collection", collection, "error", err)

			continue
		}

		var stats []bson.M
		if err = cursor.All(d.ctx, &stats); err != nil {
			logger.Error("cannot get $indexStats for collection", "database", database, "collection", collection, "error", err)

			continue
		}

		d.base.logger.Debug("indexStats", "database", database, "collection", collection)

		debugResult(d.base.logger, stats)

		for _, metric := range stats {
			indexName := fmt.Sprintf("%s", metric["name"])
			// Override the label name
			if d.overrideDescendingIndex {
				indexName = strings.ReplaceAll(fmt.Sprintf("%s", metric["name"]), "-1", "DESC")
			}

			// prefix and labels are needed to avoid duplicated metric names since the metrics are the
			// same, for different collections.
			prefix := "indexstats"
			labels := d.topologyInfo.baseLabels()
			labels["database"] = database
			labels["collection"] = collection
			labels["key_name"] = indexName

			metrics := sanitizeMetrics(metric)
			for _, metric := range makeMetrics(prefix, metrics, labels, false) {
				ch <- metric
			}
		}
	}
}

// According to specs, we should expose only this 2 metrics. 'building' might not exist.
func sanitizeMetrics(m bson.M) bson.M {
	ops := float64(0)

	if val := walkTo(m, []string{"accesses", "ops"}); val != nil {
		if f, err := asFloat64(val); err == nil {
			ops = *f
		}
	}

	filteredMetrics := bson.M{
		"accesses": bson.M{
			"ops": ops,
		},
	}

	if val := walkTo(m, []string{"building"}); val != nil {
		if f, err := asFloat64(val); err == nil {
			filteredMetrics["building"] = *f
		}
	}

	return filteredMetrics
}

var _ prometheus.Collector = (*indexstatsCollector)(nil)
