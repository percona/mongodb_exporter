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
	opCountersTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "op_counters_total"),
		"The opcounters data structure provides an overview of database operations by type and makes it possible to analyze the load on the database in more granular manner. These numbers will grow over time and in response to database use. Analyze these values over time to track database utilization",
		[]string{"type"},
		nil,
	)
)

var (
	opCountersReplTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "op_counters_repl_total"),
		"The opcountersRepl data structure, similar to the opcounters data structure, provides an overview of database replication operations by type and makes it possible to analyze the load on the replica in more granular manner. These values only appear when the current host has replication enabled",
		[]string{"type"},
		nil,
	)
)

// OpcountersStats opcounters stats
type OpcountersStats struct {
	Insert  float64 `bson:"insert"`
	Query   float64 `bson:"query"`
	Update  float64 `bson:"update"`
	Delete  float64 `bson:"delete"`
	GetMore float64 `bson:"getmore"`
	Command float64 `bson:"command"`
}

// Export exports the data to prometheus.
func (opCounters *OpcountersStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(opCountersTotalDesc, prometheus.CounterValue, opCounters.Insert, "insert")
	ch <- prometheus.MustNewConstMetric(opCountersTotalDesc, prometheus.CounterValue, opCounters.Query, "query")
	ch <- prometheus.MustNewConstMetric(opCountersTotalDesc, prometheus.CounterValue, opCounters.Update, "update")
	ch <- prometheus.MustNewConstMetric(opCountersTotalDesc, prometheus.CounterValue, opCounters.Delete, "delete")
	ch <- prometheus.MustNewConstMetric(opCountersTotalDesc, prometheus.CounterValue, opCounters.GetMore, "getmore")
	ch <- prometheus.MustNewConstMetric(opCountersTotalDesc, prometheus.CounterValue, opCounters.Command, "command")
}

// Describe describes the metrics for prometheus
func (opCounters *OpcountersStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- opCountersTotalDesc
}

// OpcountersReplStats opcounters stats
type OpcountersReplStats struct {
	Insert  float64 `bson:"insert"`
	Query   float64 `bson:"query"`
	Update  float64 `bson:"update"`
	Delete  float64 `bson:"delete"`
	GetMore float64 `bson:"getmore"`
	Command float64 `bson:"command"`
}

// Export exports the data to prometheus.
func (opCounters *OpcountersReplStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(opCountersReplTotalDesc, prometheus.CounterValue, opCounters.Insert, "insert")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotalDesc, prometheus.CounterValue, opCounters.Query, "query")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotalDesc, prometheus.CounterValue, opCounters.Update, "update")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotalDesc, prometheus.CounterValue, opCounters.Delete, "delete")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotalDesc, prometheus.CounterValue, opCounters.GetMore, "getmore")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotalDesc, prometheus.CounterValue, opCounters.Command, "command")
}

// Describe describes the metrics for prometheus
func (opCounters *OpcountersReplStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- opCountersReplTotalDesc
}
