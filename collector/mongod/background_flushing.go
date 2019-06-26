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
		"flushes is a counter that collects the number of times the database has flushed all writes to disk. This value will grow as database runs for longer periods of time",
		nil,
		nil,
	)
	backgroundFlushingtotalMillisecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "background_flushing", "total_milliseconds"),
		"The total_ms value provides the total number of milliseconds (ms) that the mongod processes have spent writing (i.e. flushing) data to disk. Because this is an absolute value, consider the value offlushes and average_ms to provide better context for this datum",
		nil,
		nil,
	)
	backgroundFlushingaverageMilliseconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "background_flushing",
		Name:      "average_milliseconds",
		Help:      `The average_ms value describes the relationship between the number of flushes and the total amount of time that the database has spent writing data to disk. The larger flushes is, the more likely this value is likely to represent a "normal," time; however, abnormal data can skew this value`,
	})
	backgroundFlushinglastMilliseconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "background_flushing",
		Name:      "last_milliseconds",
		Help:      "The value of the last_ms field is the amount of time, in milliseconds, that the last flush operation took to complete. Use this value to verify that the current performance of the server and is in line with the historical data provided by average_ms and total_ms",
	})
	backgroundFlushinglastFinishedTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "background_flushing",
		Name:      "last_finished_time",
		Help:      "The last_finished field provides a timestamp of the last completed flush operation in the ISODateformat. If this value is more than a few minutes old relative to your server’s current time and accounting for differences in time zone, restarting the database may result in some data loss",
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
