package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	assertsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "asserts_total"),
		"The asserts document reports the number of asserts on the database. While assert errors are typically uncommon, if there are non-zero values for the asserts, you should check the log file for the mongod process for more information. In many cases these errors are trivial, but are worth investigating.",
	  []string{"type"}, nil)
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
	ch <- prometheus.MustNewConstMetric(assertsTotal, prometheus.CounterValue, asserts.Regular, "regular")
	ch <- prometheus.MustNewConstMetric(assertsTotal, prometheus.CounterValue, asserts.Warning, "warning")
	ch <- prometheus.MustNewConstMetric(assertsTotal, prometheus.CounterValue, asserts.Msg, "msg")
	ch <- prometheus.MustNewConstMetric(assertsTotal, prometheus.CounterValue, asserts.User, "user")
	ch <- prometheus.MustNewConstMetric(assertsTotal, prometheus.CounterValue, asserts.Rollovers, "rollovers")
}

// Describe describes the metrics for prometheus
func (asserts *AssertsStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- assertsTotal
}
