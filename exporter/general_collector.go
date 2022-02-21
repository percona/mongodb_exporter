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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// This collector is always enabled and it is not directly related to any particular MongoDB
// command to gather stats.
type generalCollector struct {
	ctx    context.Context
	client *mongo.Client
	logger *logrus.Logger

	lock         sync.Mutex
	metricsCache []prometheus.Metric
}

func NewGeneralCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger) *generalCollector {
	return &generalCollector{
		ctx:    ctx,
		client: client,
		logger: logger,
	}
}

func (d *generalCollector) Describe(ch chan<- *prometheus.Desc) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.metricsCache = make([]prometheus.Metric, 0, defaultCacheSize)

	// This is a copy/paste of prometheus.DescribeByCollect(d, ch) with the aggreated functionality
	// to populate the metrics cache. Since on each scrape Prometheus will call Describe and inmediatelly
	// after it will call Collect, it is safe to populate the cache here.
	metrics := make(chan prometheus.Metric)
	go func() {
		d.collect(metrics)
		close(metrics)
	}()

	for m := range metrics {
		d.metricsCache = append(d.metricsCache, m) // populate the cache
		ch <- m.Desc()
	}
}

func (d *generalCollector) Collect(ch chan<- prometheus.Metric) {
	d.lock.Lock()
	defer d.lock.Unlock()

	for _, metric := range d.metricsCache {
		ch <- metric
	}
}

func (d *generalCollector) collect(ch chan<- prometheus.Metric) {
	ch <- mongodbUpMetric(d.ctx, d.client, d.logger)
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
