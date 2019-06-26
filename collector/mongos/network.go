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
	networkBytesTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "network_bytes_total"),
		"The network data structure contains data regarding MongoDB’s network use",
		[]string{"state"},
		nil,
	)
)
var (
	networkMetricsNumRequestsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "network_metrics", "num_requests_total"),
		"The numRequests field is a counter of the total number of distinct requests that the server has received. Use this value to provide context for the bytesIn and bytesOut values to ensure that MongoDB’s network utilization is consistent with expectations and application use",
		nil,
		nil,
	)
)

//NetworkStats network stats
type NetworkStats struct {
	BytesIn     float64 `bson:"bytesIn"`
	BytesOut    float64 `bson:"bytesOut"`
	NumRequests float64 `bson:"numRequests"`
}

// Export exports the data to prometheus
func (networkStats *NetworkStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(networkBytesTotalDesc, prometheus.CounterValue, networkStats.BytesIn, "in_bytes")
	ch <- prometheus.MustNewConstMetric(networkBytesTotalDesc, prometheus.CounterValue, networkStats.BytesOut, "out_bytes")
	ch <- prometheus.MustNewConstMetric(networkMetricsNumRequestsTotalDesc, prometheus.CounterValue, networkStats.NumRequests)
}

// Describe describes the metrics for prometheus
func (networkStats *NetworkStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- networkMetricsNumRequestsTotalDesc
	ch <- networkBytesTotalDesc
}
