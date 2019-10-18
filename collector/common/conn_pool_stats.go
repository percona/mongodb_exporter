package common

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// server connections -- all of these!
var (
	syncClientConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connection_sync"),
		"= connPoolStats numClientConnections",
		nil,
		nil,
	)

	numAScopedConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_scoped_sync"),
		"= connPoolStats numAScopedConnections",
		nil,
		nil,
	)

	totalInUseDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_in_use"),
		"= connPoolStats totalInUse",
		nil,
		nil,
	)

	totalAvailableDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_available"),
		"= connPoolStats totalAvailable",
		nil,
		nil,
	)

	totalCreatedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_created_total"),
		"= connPoolStats totalCreated",
		nil,
		nil,
	)
)

// ConnPoolStats keeps the data returned by the connPoolStats command.
type ConnPoolStats struct {
	SyncClientConnections float64 `bson:"numClientConnections"`
	ASScopedConnections   float64 `bson:"numAScopedConnections"`
	TotalInUse            float64 `bson:"totalInUse"`
	TotalAvailable        float64 `bson:"totalAvailable"`
	TotalCreated          float64 `bson:"totalCreated"`

	Ok float64 `bson:"ok"`
}

// Export exports the server status to be consumed by prometheus.
func (stats *ConnPoolStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(syncClientConnectionsDesc, prometheus.GaugeValue, stats.SyncClientConnections)
	ch <- prometheus.MustNewConstMetric(numAScopedConnectionsDesc, prometheus.GaugeValue, stats.ASScopedConnections)
	ch <- prometheus.MustNewConstMetric(totalInUseDesc, prometheus.GaugeValue, stats.TotalInUse)
	ch <- prometheus.MustNewConstMetric(totalAvailableDesc, prometheus.GaugeValue, stats.TotalAvailable)
	ch <- prometheus.MustNewConstMetric(totalCreatedDesc, prometheus.CounterValue, stats.TotalCreated)
}

// Describe describes the server status for prometheus.
func (stats *ConnPoolStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- syncClientConnectionsDesc
	ch <- numAScopedConnectionsDesc
	ch <- totalInUseDesc
	ch <- totalAvailableDesc
	ch <- totalCreatedDesc
}

// GetConnPoolStats returns the server connPoolStats info.
func GetConnPoolStats(client *mongo.Client) *ConnPoolStats {
	result := &ConnPoolStats{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"connPoolStats", 1}, {"recordStats", 0}}).Decode(result)
	if err != nil {
		log.Errorf("Failed to get connPoolStats: %s.", err)
		return nil
	}
	return result
}
