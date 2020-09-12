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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// This collector is always enabled and it is not directly related to any particular MongoDB
// command to gather stats.
type generalCollector struct {
	ctx    context.Context
	client *mongo.Client
}

func (d *generalCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *generalCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- mongodbUpMetric(d.ctx, d.client)
}

func mongodbUpMetric(ctx context.Context, client *mongo.Client) prometheus.Metric {
	var value float64

	if err := client.Ping(ctx, readpref.PrimaryPreferred()); err == nil {
		value = 1
	}

	d := prometheus.NewDesc("mongodb_up", "Whether MongoDB is up.", nil, nil)
	up, err := prometheus.NewConstMetric(d, prometheus.GaugeValue, value)
	if err != nil {
		return prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
	}

	return up
}

var _ prometheus.Collector = (*generalCollector)(nil)
