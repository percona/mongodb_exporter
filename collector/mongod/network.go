package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	networkBytesTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "network_bytes_total"),
		"The network data structure contains data regarding MongoDB’s network use",
	  []string{"state"}, nil)
)
var (
	networkMetricsNumRequestsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "network_metrics", "num_requests_total"),
		"The numRequests field is a counter of the total number of distinct requests that the server has received. Use this value to provide context for the bytesIn and bytesOut values to ensure that MongoDB’s network utilization is consistent with expectations and application use",
	  nil, nil)
)

//NetworkStats network stats
type NetworkStats struct {
	BytesIn     float64 `bson:"bytesIn"`
	BytesOut    float64 `bson:"bytesOut"`
	NumRequests float64 `bson:"numRequests"`
}

// Export exports the data to prometheus
func (networkStats *NetworkStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(networkBytesTotal, prometheus.CounterValue, networkStats.BytesIn, "in_bytes")
	ch <- prometheus.MustNewConstMetric(networkBytesTotal, prometheus.CounterValue, networkStats.BytesOut, "out_bytes")

	ch <- prometheus.MustNewConstMetric(networkMetricsNumRequestsTotal, prometheus.CounterValue, networkStats.NumRequests)
}

// Describe describes the metrics for prometheus
func (networkStats *NetworkStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- networkMetricsNumRequestsTotal
	ch <- networkBytesTotal
}
