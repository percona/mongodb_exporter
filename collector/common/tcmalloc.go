package common

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	tcmallocGeneralDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "tcmalloc", "generic_heap"),
		"High-level summary metricsInternal metrics from tcmalloc",
		[]string{"type"},
		nil,
	)

	tcmallocPageheapBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "tcmalloc", "pageheap_bytes"),
		"Sizes for tcpmalloc pageheaps",
		[]string{"type"},
		nil,
	)

	tcmallocPageheapCountsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "tcmalloc", "pageheap_count"),
		"Sizes for tcpmalloc pageheaps",
		[]string{"type"},
		nil,
	)

	tcmallocCacheBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "tcmalloc", "cache_bytes"),
		"Sizes for tcpmalloc caches in bytes",
		[]string{"cache", "type"},
		nil,
	)

	tcmallocAggressiveDecommitDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "tcmalloc", "aggressive_memory_decommit"),
		"Whether aggressive_memory_decommit is on",
		nil,
		nil,
	)

	tcmallocFreeBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "tcmalloc", "free_bytes"),
		"Total free bytes of tcmalloc",
		nil,
		nil,
	)
)

// TCMallocStats tracks the mem stats metrics.
type TCMallocStats struct {
	Generic GenericTCMAllocStats  `bson:"generic"`
	Details DetailedTCMallocStats `bson:"tcmalloc"`
}

// GenericTCMAllocStats tracks the mem stats generic metrics.
type GenericTCMAllocStats struct {
	CurrentAllocatedBytes float64 `bson:"current_allocated_bytes"`
	HeapSize              float64 `bson:"heap_size"`
}

// DetailedTCMallocStats tracks the mem stats detailed metrics.
type DetailedTCMallocStats struct {
	PageheapFreeBytes          float64 `bson:"pageheap_free_bytes"`
	PageheapUnmappedBytes      float64 `bson:"pageheap_unmapped_bytes"`
	PageheapComittedBytes      float64 `bson:"pageheap_committed_bytes"`
	PageheapScavengeCount      float64 `bson:"pageheap_scavenge_count"`
	PageheapCommitCount        float64 `bson:"pageheap_commit_count"`
	PageheapTotalCommitBytes   float64 `bson:"pageheap_total_commit_bytes"`
	PageheapDecommitCount      float64 `bson:"pageheap_decommit_count"`
	PageheapTotalDecommitBytes float64 `bson:"pageheap_total_decommit_bytes"`
	PageheapReserveCount       float64 `bson:"pageheap_reserve_count"`
	PageheapTotalReserveBytes  float64 `bson:"pageheap_total_reserve_bytes"`

	MaxTotalThreadCacheBytes     float64 `bson:"max_total_thread_cache_bytes"`
	CurrentTotalThreadCacheBytes float64 `bson:"current_total_thread_cache_bytes"`
	CentralCacheFreeBytes        float64 `bson:"central_cache_free_bytes"`
	TransferCacheFreeBytes       float64 `bson:"transfer_cache_free_bytes"`
	ThreadCacheFreeBytes         float64 `bson:"thread_cache_free_bytes"`

	TotalFreeBytes           float64 `bson:"total_free_bytes"`
	AggressiveMemoryDecommit float64 `bson:"aggressive_memory_decommit"`
}

// Export exports the data to prometheus.
func (m *TCMallocStats) Export(ch chan<- prometheus.Metric) {
	// Generic metrics
	ch <- prometheus.MustNewConstMetric(tcmallocGeneralDesc, prometheus.GaugeValue, m.Generic.CurrentAllocatedBytes, "allocated")
	ch <- prometheus.MustNewConstMetric(tcmallocGeneralDesc, prometheus.GaugeValue, m.Generic.HeapSize, "total")

	// Pageheap
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapBytesDesc, prometheus.GaugeValue, m.Details.PageheapFreeBytes, "free")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapBytesDesc, prometheus.GaugeValue, m.Details.PageheapUnmappedBytes, "unmapped")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapBytesDesc, prometheus.GaugeValue, m.Details.PageheapComittedBytes, "comitted")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapBytesDesc, prometheus.GaugeValue, m.Details.PageheapTotalCommitBytes, "total_commit")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapBytesDesc, prometheus.GaugeValue, m.Details.PageheapTotalDecommitBytes, "total_decommit")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapBytesDesc, prometheus.GaugeValue, m.Details.PageheapTotalReserveBytes, "total_reserve")

	ch <- prometheus.MustNewConstMetric(tcmallocPageheapCountsDesc, prometheus.GaugeValue, m.Details.PageheapScavengeCount, "scavenge")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapCountsDesc, prometheus.GaugeValue, m.Details.PageheapCommitCount, "commit")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapCountsDesc, prometheus.GaugeValue, m.Details.PageheapDecommitCount, "decommit")
	ch <- prometheus.MustNewConstMetric(tcmallocPageheapCountsDesc, prometheus.GaugeValue, m.Details.PageheapReserveCount, "reserve")

	ch <- prometheus.MustNewConstMetric(tcmallocCacheBytesDesc, prometheus.GaugeValue, m.Details.MaxTotalThreadCacheBytes, "thread_cache", "max_total")
	ch <- prometheus.MustNewConstMetric(tcmallocCacheBytesDesc, prometheus.GaugeValue, m.Details.CurrentTotalThreadCacheBytes, "thread_cache", "current_total")
	ch <- prometheus.MustNewConstMetric(tcmallocCacheBytesDesc, prometheus.GaugeValue, m.Details.CentralCacheFreeBytes, "central_cache", "free")
	ch <- prometheus.MustNewConstMetric(tcmallocCacheBytesDesc, prometheus.GaugeValue, m.Details.TransferCacheFreeBytes, "transfer_cache", "free")
	ch <- prometheus.MustNewConstMetric(tcmallocCacheBytesDesc, prometheus.GaugeValue, m.Details.ThreadCacheFreeBytes, "thread_cache", "free")

	ch <- prometheus.MustNewConstMetric(tcmallocAggressiveDecommitDesc, prometheus.CounterValue, m.Details.AggressiveMemoryDecommit)
	ch <- prometheus.MustNewConstMetric(tcmallocFreeBytesDesc, prometheus.CounterValue, m.Details.TotalFreeBytes)
}

// Describe describes the metrics for prometheus
func (m *TCMallocStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- tcmallocGeneralDesc
	ch <- tcmallocPageheapBytesDesc
	ch <- tcmallocPageheapCountsDesc
	ch <- tcmallocCacheBytesDesc
	ch <- tcmallocAggressiveDecommitDesc
	ch <- tcmallocAggressiveDecommitDesc
}
