package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	storageEngine = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "storage_engine"),
		"The storage engine used by the MongoDB instance",
		[]string{"engine"}, nil)
)

// StorageEngineStats
type StorageEngineStats struct {
	Name	string	`bson:"name"`
}

// Export exports the data to prometheus.
func (stats *StorageEngineStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(storageEngine, prometheus.CounterValue, 1, stats.Name)
}

// Describe describes the metrics for prometheus
func (stats *StorageEngineStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- storageEngine
}
