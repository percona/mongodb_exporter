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
		"Corresponds to the total number of client connections to mongo.",
		nil,
		nil,
	)

	numAScopedConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_scoped_sync"),
		"Corresponds to the number of active and stored outgoing scoped synchronous connections from the current instance to other members of the sharded cluster or replica set.",
		nil,
		nil,
	)

	totalInUseDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_in_use"),
		"Corresponds to the total number of client connections to mongo currently in use.",
		nil,
		nil,
	)

	totalAvailableDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_available"),
		"Corresponds to the total number of client connections to mongo that are currently available.",
		nil,
		nil,
	)

	totalCreatedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_created_total"),
		"Corresponds to the total number of client connections to mongo created since instance start",
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
