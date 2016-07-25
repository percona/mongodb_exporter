package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	opCountersTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "op_counters_total",
		Help:      "The opcounters data structure provides an overview of database operations by type and makes it possible to analyze the load on the database in more granular manner. These numbers will grow over time and in response to database use. Analyze these values over time to track database utilization",
	}, []string{"type"})
)
var (
	opCountersReplTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "op_counters_repl_total",
		Help:      "The opcountersRepl data structure, similar to the opcounters data structure, provides an overview of database replication operations by type and makes it possible to analyze the load on the replica in more granular manner. These values only appear when the current host has replication enabled",
	}, []string{"type"})
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
	opCountersTotal.WithLabelValues("insert").Set(opCounters.Insert)
	opCountersTotal.WithLabelValues("query").Set(opCounters.Query)
	opCountersTotal.WithLabelValues("update").Set(opCounters.Update)
	opCountersTotal.WithLabelValues("delete").Set(opCounters.Delete)
	opCountersTotal.WithLabelValues("getmore").Set(opCounters.GetMore)
	opCountersTotal.WithLabelValues("command").Set(opCounters.Command)

	opCountersTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (opCounters *OpcountersStats) Describe(ch chan<- *prometheus.Desc) {
	opCountersTotal.Describe(ch)
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
	opCountersReplTotal.WithLabelValues("insert").Set(opCounters.Insert)
	opCountersReplTotal.WithLabelValues("query").Set(opCounters.Query)
	opCountersReplTotal.WithLabelValues("update").Set(opCounters.Update)
	opCountersReplTotal.WithLabelValues("delete").Set(opCounters.Delete)
	opCountersReplTotal.WithLabelValues("getmore").Set(opCounters.GetMore)
	opCountersReplTotal.WithLabelValues("command").Set(opCounters.Command)

	opCountersReplTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (opCounters *OpcountersReplStats) Describe(ch chan<- *prometheus.Desc) {
	opCountersReplTotal.Describe(ch)
}
