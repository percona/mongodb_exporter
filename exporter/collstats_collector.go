// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package exporter

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type collstatsCollector struct {
	ctx             context.Context
	client          *mongo.Client
	collections     []string
	compatibleMode  bool
	discoveringMode bool
	logger          *logrus.Logger
	topologyInfo    labelsGetter
}

func (d *collstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *collstatsCollector) Collect(ch chan<- prometheus.Metric) {
	if d.discoveringMode {
		databases := map[string][]string{}
		for _, dbCollection := range d.collections {
			parts := strings.Split(dbCollection, ".")
			if _, ok := databases[parts[0]]; !ok {
				db := parts[0]
				databases[db], _ = d.client.Database(parts[0]).ListCollectionNames(d.ctx, bson.D{})
			}
		}

		d.collections = fromMapToSlice(databases)
	}
	for _, dbCollection := range d.collections {
		parts := strings.Split(dbCollection, ".")
		if len(parts) != 2 { //nolint:gomnd
			continue
		}

		database := parts[0]
		collection := parts[1]

		aggregation := bson.D{
			{
				Key: "$collStats", Value: bson.M{
					"latencyStats": bson.E{Key: "histograms", Value: true},
					"storageStats": bson.E{Key: "scale", Value: 1},
				},
			},
		}
		project := bson.D{
			{
				Key: "$project", Value: bson.M{
					"storageStats.wiredTiger":   0,
					"storageStats.indexDetails": 0,
				},
			},
		}

		cursor, err := d.client.Database(database).Collection(collection).Aggregate(d.ctx, mongo.Pipeline{aggregation, project})
		if err != nil {
			d.logger.Errorf("cannot get $collstats cursor for collection %s.%s: %s", database, collection, err)
			continue
		}

		var stats []bson.M
		if err = cursor.All(d.ctx, &stats); err != nil {
			d.logger.Errorf("cannot get $collstats for collection %s.%s: %s", database, collection, err)
			continue
		}

		d.logger.Debugf("$collStats metrics for %s.%s", database, collection)
		debugResult(d.logger, stats)

		// Since all collections will have the same fields, we need to use a metric prefix (db+col)
		// to differentiate metrics between collection. Labels are being set only to matke it easier
		// to filter
		prefix := database + "." + collection

		labels := d.topologyInfo.baseLabels()
		labels["database"] = database
		labels["collection"] = collection

		for _, metrics := range stats {
			for _, metric := range makeMetrics(prefix, metrics, labels, d.compatibleMode) {
				ch <- metric
			}
		}
	}
}

func fromMapToSlice(databases map[string][]string) []string {
	var collections []string
	for db, cols := range databases {
		for _, value := range cols {
			collections = append(collections, db+"."+value)
		}
	}

	return collections
}

var _ prometheus.Collector = (*collstatsCollector)(nil)
