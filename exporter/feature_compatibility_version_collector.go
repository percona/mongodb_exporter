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
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	compatibilityModeOff = false // Only used in getDiagnosticData.
)

type featureCompatibilityCollector struct {
	ctx  context.Context
	base *baseCollector

	now                  func() time.Time
	lock                 *sync.Mutex
	scrapeInterval       time.Duration
	lastScrape           time.Time
	lastCollectedMetrics primitive.M
}

// newProfileCollector creates a collector for being processed queries.
func newFeatureCompatibilityCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, scrapeInterval time.Duration) *featureCompatibilityCollector {
	return &featureCompatibilityCollector{
		ctx:                  ctx,
		base:                 newBaseCollector(client, logger.WithFields(logrus.Fields{"collector": "featureCompatibility"})),
		lock:                 &sync.Mutex{},
		scrapeInterval:       scrapeInterval,
		lastScrape:           time.Time{},
		lastCollectedMetrics: primitive.M{},
		now: func() time.Time {
			return time.Now()
		},
	}
}

func (d *featureCompatibilityCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *featureCompatibilityCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *featureCompatibilityCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "profile")()

	d.lock.Lock()
	defer d.lock.Unlock()

	if d.lastScrape.Add(d.scrapeInterval).Before(d.now()) {
		cmd := bson.D{{"getParameter", 1}, {"featureCompatibilityVersion", 1}}
		res := d.base.client.Database("admin").RunCommand(d.ctx, cmd)

		m := make(map[string]interface{})
		if err := res.Decode(&m); err != nil {
			ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
			return
		}

		d.lastScrape = d.now()

		rawValue := walkTo(m, []string{"featureCompatibilityVersion", "version"})
		if rawValue != nil {
			version, err := strconv.ParseFloat(fmt.Sprintf("%v", rawValue), 64)
			if err != nil {
				ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
				return
			}
			d.lastCollectedMetrics = primitive.M{"featureCompatibilityVersion": version}
		}
	}

	labels := map[string]string{"last_scrape": d.lastScrape.Format(time.DateTime)}
	for _, metric := range makeMetrics("fcv", d.lastCollectedMetrics, labels, compatibilityModeOff) {
		ch <- metric
	}
}
