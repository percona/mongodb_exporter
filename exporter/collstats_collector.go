// mnogo_exporter
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
	"encoding/json"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type collstatsCollector struct {
	ctx         context.Context
	client      *mongo.Client
	collections []string
}

func (d *collstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *collstatsCollector) Collect(ch chan<- prometheus.Metric) {
	for _, dbCollection := range d.collections {
		parts := strings.Split(dbCollection, ".")
		if len(parts) != 2 { //nolint
			continue
		}

		db := parts[0]
		col := parts[1]

		cmd := bson.D{{Key: "collStats", Value: col}}
		res := d.client.Database(db).RunCommand(d.ctx, cmd)

		var m bson.M
		if err := res.Decode(&m); err != nil {
			ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
			continue
		}

		if buf, err := json.MarshalIndent(m, "", "    "); err == nil {
			logrus.Debugf(string(buf))
		}

		labels := map[string]string{
			"database":   db,
			"collection": col,
		}

		// Since all collections will have the same fields, we need to use a metric prefix (db+col)
		// to differentiate metrics between collection. Labels are being set only to matke it easier
		// to filter
		for _, metric := range makeMetrics(db+"."+col, m, labels) {
			ch <- metric
		}
	}
}
