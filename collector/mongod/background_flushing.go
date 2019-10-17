// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongod

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	backgroundFlushingflushesTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "background_flushing", "flushes_total"),
		"source = serverStatus backgroundFlushing.flushes",
		nil,
		nil,
	)
	backgroundFlushingtotalMillisecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "background_flushing", "total_milliseconds"),
		"source = serverStatus backgroundFlushing.total_ms",
		nil,
		nil,
	)
	backgroundFlushingaverageMilliseconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "background_flushing",
		Name:      "average_milliseconds",
		Help:      "source = serverStatus backgroundFlushing.average_ms",
	})
	backgroundFlushinglastMilliseconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "background_flushing",
		Name:      "last_milliseconds",
		Help:      "source = serverStatus backgroundFlushing.last_ms",
	})
	backgroundFlushinglastFinishedTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "background_flushing",
		Name:      "last_finished_time",
		Help:      "source = serverStatus backgroundFlushing.last_finished",
	})
)

// FlushStats is the flush stats metrics
type FlushStats struct {
	Flushes      float64   `bson:"flushes"`
	TotalMs      float64   `bson:"total_ms"`
	AverageMs    float64   `bson:"average_ms"`
	LastMs       float64   `bson:"last_ms"`
	LastFinished time.Time `bson:"last_finished"`
}

// Export exports the metrics for prometheus.
func (flushStats *FlushStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(backgroundFlushingflushesTotalDesc, prometheus.CounterValue, flushStats.Flushes)
	ch <- prometheus.MustNewConstMetric(backgroundFlushingtotalMillisecondsDesc, prometheus.CounterValue, flushStats.TotalMs)

	backgroundFlushingaverageMilliseconds.Set(flushStats.AverageMs)
	backgroundFlushinglastMilliseconds.Set(flushStats.LastMs)
	backgroundFlushinglastFinishedTime.Set(float64(flushStats.LastFinished.Unix()))

	backgroundFlushingaverageMilliseconds.Collect(ch)
	backgroundFlushinglastMilliseconds.Collect(ch)
	backgroundFlushinglastFinishedTime.Collect(ch)
}

// Describe describes the metrics for prometheus
func (flushStats *FlushStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- backgroundFlushingflushesTotalDesc
	ch <- backgroundFlushingtotalMillisecondsDesc
	backgroundFlushingaverageMilliseconds.Describe(ch)
	backgroundFlushinglastMilliseconds.Describe(ch)
	backgroundFlushinglastFinishedTime.Describe(ch)
}
