package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	networkBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "network_bytes_total",
		Help:      "The network data structure contains data regarding MongoDB’s network use",
	}, []string{"state"})
)
var (
	networkMetricsNumRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "network_metrics",
		Name:      "num_requests_total",
		Help:      "The numRequests field is a counter of the total number of distinct requests that the server has received. Use this value to provide context for the bytesIn and bytesOut values to ensure that MongoDB’s network utilization is consistent with expectations and application use",
	})
)

//NetworkStats network stats
type NetworkStats struct {
	BytesIn     float64 `bson:"bytesIn"`
	BytesOut    float64 `bson:"bytesOut"`
	NumRequests float64 `bson:"numRequests"`
}

// Export exports the data to prometheus
func (networkStats *NetworkStats) Export(ch chan<- prometheus.Metric) {
	networkBytesTotal.WithLabelValues("in_bytes").Set(networkStats.BytesIn)
	networkBytesTotal.WithLabelValues("out_bytes").Set(networkStats.BytesOut)

	networkMetricsNumRequestsTotal.Set(networkStats.NumRequests)

	networkMetricsNumRequestsTotal.Collect(ch)
	networkBytesTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (networkStats *NetworkStats) Describe(ch chan<- *prometheus.Desc) {
	networkMetricsNumRequestsTotal.Describe(ch)
	networkBytesTotal.Describe(ch)
}
