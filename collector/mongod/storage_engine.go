package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	storageEngine = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "storage_engine",
		Help:      "The storage engine used by the MongoDB instance",
	}, []string{"engine"})
)

// StorageEngineStats
type StorageEngineStats struct {
	Name string `bson:"name"`
}

// Export exports the data to prometheus.
func (stats *StorageEngineStats) Export(ch chan<- prometheus.Metric) {
	storageEngine.WithLabelValues(stats.Name).Set(1)
	storageEngine.Collect(ch)
}

// Describe describes the metrics for prometheus
func (stats *StorageEngineStats) Describe(ch chan<- *prometheus.Desc) {
	storageEngine.Describe(ch)
}
