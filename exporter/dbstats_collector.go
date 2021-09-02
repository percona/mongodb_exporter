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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type dbstatsCollector struct {
	ctx            context.Context
	client         *mongo.Client
	compatibleMode bool
	logger         *logrus.Logger
	topologyInfo   labelsGetter
}

func (d *dbstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *dbstatsCollector) Collect(ch chan<- prometheus.Metric) {
	// List all databases names
	dbNames, err := d.client.ListDatabaseNames(d.ctx, bson.M{})
	if err != nil {
		d.logger.Errorf("Failed to get database names: %s", err)
		return
	}
	d.logger.Debugf("getting stats for databases: %v", dbNames)
	for _, db := range dbNames {
		var dbStats bson.M
		cmd := bson.D{{Key: "dbStats", Value: 1}, {Key: "scale", Value: 1}}
		r := d.client.Database(db).RunCommand(d.ctx, cmd)
		err := r.Decode(&dbStats)
		if err != nil {
			d.logger.Errorf("Failed to get $dbstats for database %s: %s", db, err)
			continue
		}

		d.logger.Debugf("$dbStats metrics for %s", db)
		debugResult(d.logger, dbStats)

		prefix := "dbstats"

		labels := d.topologyInfo.baseLabels()

		// Since all dbstats will have the same fields, we need to use a label
		// to differentiate metrics between different databases.
		labels["database"] = db

		for _, metric := range makeMetrics(prefix, dbStats, labels, d.compatibleMode) {
			ch <- metric
		}
	}
}

var _ prometheus.Collector = (*dbstatsCollector)(nil)
