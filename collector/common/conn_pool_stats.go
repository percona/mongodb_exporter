package collector_common

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// server connections -- all of these!
var (
	syncClientConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "connpoolstats",
		Name:      "connection_sync",
		Help:      "Corresponds to the total number of client connections to mongo.",
	})

	numAScopedConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "connpoolstats",
		Name:      "connections_scoped_sync",
		Help:      "Corresponds to the number of active and stored outgoing scoped synchronous connections from the current instance to other members of the sharded cluster or replica set.",
	})

	totalInUse = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "connpoolstats",
		Name:      "connections_in_use",
		Help:      "Corresponds to the total number of client connections to mongo currently in use.",
	})

	totalAvailable = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "connpoolstats",
		Name:      "connections_available",
		Help:      "Corresponds to the total number of client connections to mongo that are currently available.",
	})

	totalCreatedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connpoolstats", "connections_created_total"),
		"Corresponds to the total number of client connections to mongo created since instance start",
		nil,
		nil,
	)
)

// ServerStatus keeps the data returned by the serverStatus() method.
type ConnPoolStats struct {
	SyncClientConnections float64 `bson:"numClientConnections"`
	ASScopedConnections   float64 `bson:"numAScopedConnections"`
	TotalInUse            float64 `bson:"totalInUse"`
	TotalAvailable        float64 `bson:"totalAvailable"`
	TotalCreated          float64 `bson:"totalCreated"`
}

// Export exports the server status to be consumed by prometheus.
func (stats *ConnPoolStats) Export(ch chan<- prometheus.Metric) {
	syncClientConnections.Set(stats.SyncClientConnections)
	syncClientConnections.Collect(ch)

	numAScopedConnections.Set(stats.ASScopedConnections)
	numAScopedConnections.Collect(ch)

	totalInUse.Set(stats.TotalInUse)
	totalInUse.Collect(ch)

	totalAvailable.Set(stats.TotalAvailable)
	totalAvailable.Collect(ch)

	ch <- prometheus.MustNewConstMetric(totalCreatedDesc, prometheus.CounterValue, stats.TotalCreated)
}

// Describe describes the server status for prometheus.
func (stats *ConnPoolStats) Describe(ch chan<- *prometheus.Desc) {
	syncClientConnections.Describe(ch)
	numAScopedConnections.Describe(ch)
	totalInUse.Describe(ch)
	totalAvailable.Describe(ch)
	ch <- totalCreatedDesc
}

// GetServerStatus returns the server status info.
func GetConnPoolStats(client *mongo.Client) *ConnPoolStats {
	result := &ConnPoolStats{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"connPoolStats", 1}, {"recordStats", 0}}).Decode(result)
	if err != nil {
		log.Error("Failed to get server status.")
		return nil
	}
	return result
}
