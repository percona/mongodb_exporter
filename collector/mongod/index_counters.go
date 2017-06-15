package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	indexCountersMissRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "index_counters",
		Name:      "miss_ratio",
		Help:      "The missRatio value is the ratio of hits to misses. This value is typically 0 or approaching 0",
	})
)

var (
	indexCountersTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "index_counters_total"),
		"Total indexes by type",
	  []string{"type"}, nil)
)

//IndexCounterStats index counter stats
type IndexCounterStats struct {
	Accesses  float64 `bson:"accesses"`
	Hits      float64 `bson:"hits"`
	Misses    float64 `bson:"misses"`
	Resets    float64 `bson:"resets"`
	MissRatio float64 `bson:"missRatio"`
}

// Export exports the data to prometheus.
func (indexCountersStats *IndexCounterStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(indexCountersTotal, prometheus.CounterValue, indexCountersStats.Accesses, "accesses")
	ch <- prometheus.MustNewConstMetric(indexCountersTotal, prometheus.CounterValue, indexCountersStats.Hits, "hits")
	ch <- prometheus.MustNewConstMetric(indexCountersTotal, prometheus.CounterValue, indexCountersStats.Misses, "misses")
	ch <- prometheus.MustNewConstMetric(indexCountersTotal, prometheus.CounterValue, indexCountersStats.Resets, "resets")

	indexCountersMissRatio.Set(indexCountersStats.MissRatio)

	indexCountersMissRatio.Collect(ch)

}

// Describe describes the metrics for prometheus
func (indexCountersStats *IndexCounterStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- indexCountersTotal
	indexCountersMissRatio.Describe(ch)
}
