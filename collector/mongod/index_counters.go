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
	"github.com/prometheus/client_golang/prometheus"
)

var (
	indexCountersMissRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "index_counters",
		Name:      "miss_ratio",
		Help:      "The missRatio value is the ratio of hits to misses. This value is typically 0 or approaching 0",
	})
)

var (
	indexCountersTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "index_counters_total"),
		"Total indexes by type",
		[]string{"type"},
		nil,
	)
)

//IndexCounterStats index counter stats
type IndexCounterStats struct {
	Accesses  float64 `bson:"accesses"`
	Hits      float64 `bson:"hits"`
	Misses    float64 `bson:"misses"`
	Resets    float64 `bson:"resets"`
	MissRatio float64 `bson:"missRatio"`
}

// Export exports the data to prometheus.
func (indexCountersStats *IndexCounterStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(indexCountersTotalDesc, prometheus.CounterValue, indexCountersStats.Accesses, "accesses")
	ch <- prometheus.MustNewConstMetric(indexCountersTotalDesc, prometheus.CounterValue, indexCountersStats.Hits, "hits")
	ch <- prometheus.MustNewConstMetric(indexCountersTotalDesc, prometheus.CounterValue, indexCountersStats.Misses, "misses")
	ch <- prometheus.MustNewConstMetric(indexCountersTotalDesc, prometheus.CounterValue, indexCountersStats.Resets, "resets")

	indexCountersMissRatio.Set(indexCountersStats.MissRatio)

	indexCountersMissRatio.Collect(ch)

}

// Describe describes the metrics for prometheus
func (indexCountersStats *IndexCounterStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- indexCountersTotalDesc
	indexCountersMissRatio.Describe(ch)
}
