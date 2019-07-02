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

package mongos

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsCursorTimedOutTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_cursor", "timed_out_total"),
		"timedOut provides the total number of cursors that have timed out since the server process started. If this number is large or growing at a regular rate, this may indicate an application error",
		nil,
		nil,
	)
)
var (
	metricsCursorOpen = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "metrics_cursor_open",
		Help:      "The open is an embedded document that contains data regarding open cursors",
	}, []string{"state"})
)
var (
	metricsGetLastErrorWtimeNumTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "metrics_get_last_error_wtime",
		Name:      "num_total",
		Help:      "num reports the total number of getLastError operations with a specified write concern (i.e. w) that wait for one or more members of a replica set to acknowledge the write operation (i.e. a w value greater than 1.)",
	})
	metricsGetLastErrorWtimeTotalMillisecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_get_last_error_wtime", "total_milliseconds"),
		"total_millis reports the total amount of time in milliseconds that the mongod has spent performing getLastError operations with write concern (i.e. w) that wait for one or more members of a replica set to acknowledge the write operation (i.e. a w value greater than 1.)",
		nil,
		nil,
	)
)
var (
	metricsGetLastErrorWtimeoutsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_get_last_error", "wtimeouts_total"),
		"wtimeouts reports the number of times that write concern operations have timed out as a result of the wtimeout threshold to getLastError.",
		nil,
		nil,
	)
)

// BenchmarkStats is bechmark info about an operation.
type BenchmarkStats struct {
	Num         float64 `bson:"num"`
	TotalMillis float64 `bson:"totalMillis"`
}

// GetLastErrorStats are the last error stats.
type GetLastErrorStats struct {
	Wtimeouts float64         `bson:"wtimeouts"`
	Wtime     *BenchmarkStats `bson:"wtime"`
}

// Export exposes the get last error stats.
func (getLastErrorStats *GetLastErrorStats) Export(ch chan<- prometheus.Metric) {
	metricsGetLastErrorWtimeNumTotal.Set(getLastErrorStats.Wtime.Num)
	ch <- prometheus.MustNewConstMetric(metricsGetLastErrorWtimeTotalMillisecondsDesc, prometheus.CounterValue, getLastErrorStats.Wtime.TotalMillis)
	ch <- prometheus.MustNewConstMetric(metricsGetLastErrorWtimeoutsTotalDesc, prometheus.CounterValue, getLastErrorStats.Wtimeouts)
}

// CursorStatsOpen are the stats for open cursors
type CursorStatsOpen struct {
	NoTimeout float64 `bson:"noTimeout"`
	Pinned    float64 `bson:"pinned"`
	Total     float64 `bson:"total"`
}

// CursorStats are the stats for cursors
type CursorStats struct {
	TimedOut float64          `bson:"timedOut"`
	Open     *CursorStatsOpen `bson:"open"`
}

// Export exports the cursor stats.
func (cursorStats *CursorStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsCursorTimedOutTotalDesc, prometheus.CounterValue, cursorStats.TimedOut)
	metricsCursorOpen.WithLabelValues("noTimeout").Set(cursorStats.Open.NoTimeout)
	metricsCursorOpen.WithLabelValues("pinned").Set(cursorStats.Open.Pinned)
	metricsCursorOpen.WithLabelValues("total").Set(cursorStats.Open.Total)
}

// MetricsStats are all stats associated with metrics of the system
type MetricsStats struct {
	GetLastError *GetLastErrorStats `bson:"getLastError"`
	Cursor       *CursorStats       `bson:"cursor"`
}

// Export exports the metrics stats.
func (metricsStats *MetricsStats) Export(ch chan<- prometheus.Metric) {
	if metricsStats.GetLastError != nil {
		metricsStats.GetLastError.Export(ch)
	}
	if metricsStats.Cursor != nil {
		metricsStats.Cursor.Export(ch)
	}

	metricsCursorOpen.Collect(ch)
	metricsGetLastErrorWtimeNumTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (metricsStats *MetricsStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- metricsCursorTimedOutTotalDesc
	metricsCursorOpen.Describe(ch)
	metricsGetLastErrorWtimeNumTotal.Describe(ch)
	ch <- metricsGetLastErrorWtimeTotalMillisecondsDesc
	ch <- metricsGetLastErrorWtimeoutsTotalDesc
}
