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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// This collector is always enabled and it is not directly related to any particular MongoDB
// command to gather stats.
type generalCollector struct {
	ctx  context.Context
	base *baseCollector
}

// newGeneralCollector creates a collector for MongoDB connectivity status.
func newGeneralCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger) *generalCollector {
	return &generalCollector{
		ctx:  ctx,
		base: newBaseCollector(client, logger),
	}
}

func (d *generalCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *generalCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *generalCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "general")()
	ch <- mongodbUpMetric(d.ctx, d.base.client, d.base.logger)
}

func mongodbUpMetric(ctx context.Context, client *mongo.Client, log *logrus.Logger) prometheus.Metric {
	var value float64

	if client != nil {
		if err := client.Ping(ctx, readpref.PrimaryPreferred()); err == nil {
			value = 1
		} else {
			log.Errorf("error while checking mongodb connection: %s. mongo_up is set to 0", err)
		}
	}

	d := prometheus.NewDesc("mongodb_up", "Whether MongoDB is up.", nil, nil)

	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value)
}

var _ prometheus.Collector = (*generalCollector)(nil)
