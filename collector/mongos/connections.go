package collector_mongos

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	connections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "connections",
		Help:      "The connections sub document data regarding the current status of incoming connections and availability of the database server. Use these values to assess the current load and capacity requirements of the server",
	}, []string{"state"})
)
var (
	connectionsMetricsCreatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "connections_metrics",
		Name:      "created_total",
		Help:      "totalCreated provides a count of all incoming connections created to the server. This number includes connections that have since closed",
	})
)

// ConnectionStats are connections metrics
type ConnectionStats struct {
	Current      float64 `bson:"current"`
	Available    float64 `bson:"available"`
	TotalCreated float64 `bson:"totalCreated"`
}

// Export exports the data to prometheus.
func (connectionStats *ConnectionStats) Export(ch chan<- prometheus.Metric) {
	connections.WithLabelValues("current").Set(connectionStats.Current)
	connections.WithLabelValues("available").Set(connectionStats.Available)
	connections.Collect(ch)

	connectionsMetricsCreatedTotal.Set(connectionStats.TotalCreated)
	connectionsMetricsCreatedTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (connectionStats *ConnectionStats) Describe(ch chan<- *prometheus.Desc) {
	connections.Describe(ch)
	connectionsMetricsCreatedTotal.Describe(ch)
}
