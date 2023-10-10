// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

type baseCollector struct {
	client *mongo.Client
	logger *logrus.Logger

	lock         sync.Mutex
	metricsCache []prometheus.Metric
}

// newBaseCollector creates a skeletal collector, which is used to create other collectors.
func newBaseCollector(client *mongo.Client, logger *logrus.Logger) *baseCollector {
	return &baseCollector{
		client: client,
		logger: logger,
	}
}

func (d *baseCollector) Describe(ctx context.Context, ch chan<- *prometheus.Desc, collect func(mCh chan<- prometheus.Metric)) {
	select {
	case <-ctx.Done():
		// don't interrupt, let mongodb_up metric to be registered if on timeout we still don't have client connected
		if d.client != nil {
			return
		}
	default:
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	d.metricsCache = make([]prometheus.Metric, 0, defaultCacheSize)

	// This is a copy/paste of prometheus.DescribeByCollect(d, ch) with the aggreated functionality
	// to populate the metrics cache. Since on each scrape Prometheus will call Describe and inmediatelly
	// after it will call Collect, it is safe to populate the cache here.
	metrics := make(chan prometheus.Metric)
	go func() {
		collect(metrics)
		close(metrics)
	}()

	for m := range metrics {
		d.metricsCache = append(d.metricsCache, m) // populate the cache
		ch <- m.Desc()
	}
}

func (d *baseCollector) Collect(ch chan<- prometheus.Metric) {
	d.lock.Lock()
	defer d.lock.Unlock()

	for _, metric := range d.metricsCache {
		ch <- metric
	}
}
