package mongod

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TopStatus represents top metrics
type TopStatus struct {
	TopStats TopStatsMap `bson:"totals,omitempty"`
}

// GetTopStats fetches top stats
func GetTopStats(client *mongo.Client) (*TopStatus, error) {
	results := &TopStatus{} // TODO: Not working as of "note" field in mongodb result...
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"top", 1}}).Decode(&results)
	return results, err
}

// Export exports metrics to Prometheus
func (status *TopStatus) Export(ch chan<- prometheus.Metric) {
	status.TopStats.Export(ch)
}

// GetTopStatus fetches top stats
func GetTopStatus(client *mongo.Client) *TopStatus {
	topStatus, err := GetTopStats(client)
	if err != nil {
		log.Debug("Failed to get top status.")
		return nil
	}

	return topStatus
}
