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

package common

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	memory = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "memory",
		Help:      "The mem data structure holds information regarding the target system architecture of mongod and current memory use",
	}, []string{"type"})
)

// MemStats tracks the mem stats metrics.
type MemStats struct {
	Bits              float64 `bson:"bits"`
	Resident          float64 `bson:"resident"`
	Virtual           float64 `bson:"virtual"`
	Mapped            float64 `bson:"mapped"`
	MappedWithJournal float64 `bson:"mappedWithJournal"`
}

// Export exports the data to prometheus.
func (memStats *MemStats) Export(ch chan<- prometheus.Metric) {
	memory.WithLabelValues("resident").Set(memStats.Resident)
	memory.WithLabelValues("virtual").Set(memStats.Virtual)
	memory.WithLabelValues("mapped").Set(memStats.Mapped)
	memory.WithLabelValues("mapped_with_journal").Set(memStats.MappedWithJournal)
	memory.Collect(ch)
}

// Describe describes the metrics for prometheus
func (memStats *MemStats) Describe(ch chan<- *prometheus.Desc) {
	memory.Describe(ch)
}
