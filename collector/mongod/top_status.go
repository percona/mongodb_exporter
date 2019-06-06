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
	raw := &TopStatusRaw{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"top", 1}}).Decode(&raw)
	results := raw.TopStatus()
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

// TopStatusRaw represents top metrics in raw format.
// This structure needed because "top" command returns and "note" field with string value, which can't be decoded to "TopStats".
type TopStatusRaw struct {
	TopStats map[string]bson.Raw `bson:"totals,omitempty"`
}

// TopStatus converts TopStatusRaw to TopStatus.
func (tsr *TopStatusRaw) TopStatus() *TopStatus {
	topStatus := &TopStatus{
		TopStats: make(TopStatsMap),
	}

	for name, value := range tsr.TopStats {
		if name == "note" {
			continue
		}
		tmp := TopStats{}
		err := bson.Unmarshal(value, &tmp)
		if err == nil {
			topStatus.TopStats[name] = tmp
		}
	}

	return topStatus
}
