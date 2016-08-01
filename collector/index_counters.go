package collector

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
	indexCountersTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "index_counters_total",
		Help:      "Total indexes by type",
	}, []string{"type"})
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
	indexCountersTotal.WithLabelValues("accesses").Set(indexCountersStats.Accesses)
	indexCountersTotal.WithLabelValues("hits").Set(indexCountersStats.Hits)
	indexCountersTotal.WithLabelValues("misses").Set(indexCountersStats.Misses)
	indexCountersTotal.WithLabelValues("resets").Set(indexCountersStats.Resets)

	indexCountersMissRatio.Set(indexCountersStats.MissRatio)

	indexCountersTotal.Collect(ch)
	indexCountersMissRatio.Collect(ch)

}

// Describe describes the metrics for prometheus
func (indexCountersStats *IndexCounterStats) Describe(ch chan<- *prometheus.Desc) {
	indexCountersTotal.Describe(ch)
	indexCountersMissRatio.Describe(ch)
}
