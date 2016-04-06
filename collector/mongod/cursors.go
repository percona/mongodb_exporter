package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	cursorsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "cursors",
		Help:      "The cursors data structure contains data regarding cursor state and use",
	}, []string{"state"})
)

// Cursors are the cursor metrics
type Cursors struct {
	TotalOpen      float64 `bson:"totalOpen"`
	TimeOut        float64 `bson:"timedOut"`
	TotalNoTimeout float64 `bson:"totalNoTimeout"`
	Pinned         float64 `bson:"pinned"`
}

// Export exports the data to prometheus.
func (cursors *Cursors) Export(ch chan<- prometheus.Metric) {
	cursorsGauge.WithLabelValues("total_open").Set(cursors.TotalOpen)
	cursorsGauge.WithLabelValues("timed_out").Set(cursors.TimeOut)
	cursorsGauge.WithLabelValues("total_no_timeout").Set(cursors.TotalNoTimeout)
	cursorsGauge.WithLabelValues("pinned").Set(cursors.Pinned)
	cursorsGauge.Collect(ch)
}

// Describe describes the metrics for prometheus
func (cursors *Cursors) Describe(ch chan<- *prometheus.Desc) {
	cursorsGauge.Describe(ch)
}
