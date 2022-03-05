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
	"fmt"

	"github.com/kr/pretty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type topCollector struct {
	ctx            context.Context
	client         *mongo.Client
	compatibleMode bool
	logger         *logrus.Logger
	topologyInfo   labelsGetter
}

type topResponse struct {
	Totals map[string]interface{} `bson:"totals"`
}

func (d *topCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *topCollector) Collect(ch chan<- prometheus.Metric) {
	cmd := bson.D{{Key: "top", Value: "1"}}
	res := d.client.Database("admin").RunCommand(d.ctx, cmd)

	var m primitive.M
	if err := res.Decode(&m); err != nil {
		ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
		return
	}

	logrus.Debug("top result:")
	debugResult(d.logger, m)

	totals, ok := m["totals"].(primitive.M)
	if !ok {
		panic(fmt.Errorf("dsddd"))
	}

	pretty.Println(m)
	for namespace, metrics := range totals {
		fmt.Println(namespace, metrics)
		labels := d.topologyInfo.baseLabels()
		labels["namespace"] = namespace
		pretty.Println(metrics)
		mm, ok := metrics.(primitive.M)
		if !ok {
			continue
		}
		pretty.Println(mm)
		for _, metric := range makeMetrics("top", mm, labels, d.compatibleMode) {
			ch <- metric
		}
	}
}
