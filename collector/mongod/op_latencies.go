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
	"strconv"
)

var (
	opLatenciesTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "op_latencies_latency_total",
		Help:      "op latencies statistics in microseconds of mongod",
	}, []string{"type"})

	opLatenciesCountTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "op_latencies_ops_total",
		Help:      "op latencies ops total statistics of mongod",
	}, []string{"type"})

	opLatenciesHistogram = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "op_latencies_histogram",
		Help:      "op latencies histogram statistics of mongod",
	}, []string{"type", "micros"})
)

// HistBucket describes a item of op latencies histogram
type HistBucket struct {
	Micros int64   `bson:"micros"`
	Count  float64 `bson:"count"`
}

// LatencyStat describes op latencies statistic
type LatencyStat struct {
	Histogram []HistBucket `bson:"histogram"`
	Latency   float64      `bson:"latency"`
	Ops       float64      `bson:"ops"`
}

// Update update each metric
func (ls *LatencyStat) Update(op string) {
	if ls.Histogram != nil {
		for _, bucket := range ls.Histogram {
			opLatenciesHistogram.WithLabelValues(op, strconv.FormatInt(bucket.Micros, 10)).Set(bucket.Count)
		}
	}
	opLatenciesTotal.WithLabelValues(op).Set(ls.Latency)
	opLatenciesCountTotal.WithLabelValues(op).Set(ls.Ops)
}

// OpLatenciesStat includes reads, writes and commands latency statistic
type OpLatenciesStat struct {
	Reads    *LatencyStat `bson:"reads"`
	Writes   *LatencyStat `bson:"writes"`
	Commands *LatencyStat `bson:"commands"`
}

// Export exports metrics to Prometheus
func (stat *OpLatenciesStat) Export(ch chan<- prometheus.Metric) {
	if stat.Reads != nil {
		stat.Reads.Update("read")
	}
	if stat.Writes != nil {
		stat.Writes.Update("write")
	}
	if stat.Commands != nil {
		stat.Commands.Update("command")
	}

	opLatenciesTotal.Collect(ch)
	opLatenciesCountTotal.Collect(ch)
	opLatenciesHistogram.Collect(ch)
}

// Describe describes the metrics for prometheus
func (stat *OpLatenciesStat) Describe(ch chan<- *prometheus.Desc) {
	opLatenciesTotal.Describe(ch)
	opLatenciesCountTotal.Describe(ch)
	opLatenciesHistogram.Describe(ch)
}
