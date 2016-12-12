package collector_mongos

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	opCountersTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "op_counters_total"),
		"The opcounters data structure provides an overview of database operations by type and makes it possible to analyze the load on the database in more granular manner. These numbers will grow over time and in response to database use. Analyze these values over time to track database utilization",
	  []string{"type"}, nil)
)
var (
	opCountersReplTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "op_counters_repl_total"),
		"The opcountersRepl data structure, similar to the opcounters data structure, provides an overview of database replication operations by type and makes it possible to analyze the load on the replica in more granular manner. These values only appear when the current host has replication enabled",
	  []string{"type"}, nil)
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
	ch <- prometheus.MustNewConstMetric(opCountersTotal, prometheus.CounterValue, opCounters.Insert, "insert")
	ch <- prometheus.MustNewConstMetric(opCountersTotal, prometheus.CounterValue, opCounters.Query, "query")
	ch <- prometheus.MustNewConstMetric(opCountersTotal, prometheus.CounterValue, opCounters.Update, "update")
	ch <- prometheus.MustNewConstMetric(opCountersTotal, prometheus.CounterValue, opCounters.Delete, "delete")
	ch <- prometheus.MustNewConstMetric(opCountersTotal, prometheus.CounterValue, opCounters.GetMore, "getmore")
	ch <- prometheus.MustNewConstMetric(opCountersTotal, prometheus.CounterValue, opCounters.Command, "command")
}

// Describe describes the metrics for prometheus
func (opCounters *OpcountersStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- opCountersTotal
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
	ch <- prometheus.MustNewConstMetric(opCountersReplTotal, prometheus.CounterValue, opCounters.Insert, "insert")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotal, prometheus.CounterValue, opCounters.Query, "query")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotal, prometheus.CounterValue, opCounters.Update, "update")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotal, prometheus.CounterValue, opCounters.Delete, "delete")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotal, prometheus.CounterValue, opCounters.GetMore, "getmore")
	ch <- prometheus.MustNewConstMetric(opCountersReplTotal, prometheus.CounterValue, opCounters.Command, "command")
}

// Describe describes the metrics for prometheus
func (opCounters *OpcountersReplStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- opCountersReplTotal
}
