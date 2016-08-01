package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	extraInfopageFaultsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "extra_info",
		Name:      "page_faults_total",
		Help:      "The page_faults Reports the total number of page faults that require disk operations. Page faults refer to operations that require the database server to access data which isn’t available in active memory. The page_faults counter may increase dramatically during moments of poor performance and may correlate with limited memory environments and larger data sets. Limited and sporadic page faults do not necessarily indicate an issue",
	})
	extraInfoheapUsageBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "extra_info",
		Name:      "heap_usage_bytes",
		Help:      "The heap_usage_bytes field is only available on Unix/Linux systems, and reports the total size in bytes of heap space used by the database process",
	})
)

// ExtraInfo has extra info metrics
type ExtraInfo struct {
	HeapUsageBytes float64 `bson:"heap_usage_bytes"`
	PageFaults     float64 `bson:"page_faults"`
}

// Export exports the metrics to prometheus.
func (extraInfo *ExtraInfo) Export(ch chan<- prometheus.Metric) {
	extraInfoheapUsageBytes.Set(extraInfo.HeapUsageBytes)
	extraInfopageFaultsTotal.Set(extraInfo.PageFaults)

	extraInfoheapUsageBytes.Collect(ch)
	extraInfopageFaultsTotal.Collect(ch)

}

// Describe describes the metrics for prometheus
func (extraInfo *ExtraInfo) Describe(ch chan<- *prometheus.Desc) {
	extraInfoheapUsageBytes.Describe(ch)
	extraInfopageFaultsTotal.Describe(ch)
}
