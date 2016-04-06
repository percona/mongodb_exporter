package collector_mongos

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	assertsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "asserts_total",
		Help:      "The asserts document reports the number of asserts on the database. While assert errors are typically uncommon, if there are non-zero values for the asserts, you should check the log file for the mongod process for more information. In many cases these errors are trivial, but are worth investigating.",
	}, []string{"type"})
)

// AssertsStats has the assets metrics
type AssertsStats struct {
	Regular   float64 `bson:"regular"`
	Warning   float64 `bson:"warning"`
	Msg       float64 `bson:"msg"`
	User      float64 `bson:"user"`
	Rollovers float64 `bson:"rollovers"`
}

// Export exports the metrics to prometheus.
func (asserts *AssertsStats) Export(ch chan<- prometheus.Metric) {
	assertsTotal.WithLabelValues("regular").Set(asserts.Regular)
	assertsTotal.WithLabelValues("warning").Set(asserts.Warning)
	assertsTotal.WithLabelValues("msg").Set(asserts.Msg)
	assertsTotal.WithLabelValues("user").Set(asserts.User)
	assertsTotal.WithLabelValues("rollovers").Set(asserts.Rollovers)
	assertsTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (asserts *AssertsStats) Describe(ch chan<- *prometheus.Desc) {
	assertsTotal.Describe(ch)
}
