package collector_mongod

import (
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	topTimeSecondsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "top_time_seconds_total",
		Help:      "The top command provides operation time, in seconds, for each database collection",
	}, []string{"type", "database", "collection"})
	topCountTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "top_count_total",
		Help:      "The top command provides operation count for each database collection",
	}, []string{"type", "database", "collection"})
)

// TopStatsMap is a map of top stats
type TopStatsMap map[string]TopStats

// TopcountersStats topcounters stats
type TopcounterStats struct {
	Time  float64 `bson:"time"`
	Count float64 `bson:"count"`
}

// TopCollectionStats top collection stats
type TopStats struct {
	Total     TopcounterStats `bson:"total"`
	ReadLock  TopcounterStats `bson:"readLock"`
	WriteLock TopcounterStats `bson:"writeLock"`
	Queries   TopcounterStats `bson:"queries"`
	GetMore   TopcounterStats `bson:"getmore"`
	Insert    TopcounterStats `bson:"insert"`
	Update    TopcounterStats `bson:"update"`
	Remove    TopcounterStats `bson:"remove"`
	Commands  TopcounterStats `bson:"commands"`
}

// Export exports the data to prometheus.
func (topStats TopStatsMap) Export(ch chan<- prometheus.Metric) {

	for collectionNamespace, topStat := range topStats {

		namespace := strings.Split(collectionNamespace, ".")
		database := namespace[0]
		collection := strings.Join(namespace[1:], ".")

		topStatTypes := reflect.TypeOf(topStat)
		topStatValues := reflect.ValueOf(topStat)

		for i := 0; i < topStatValues.NumField(); i++ {

			metric_type := topStatTypes.Field(i).Name

			op_count := topStatValues.Field(i).Field(1).Float()

			op_time_microsecond := topStatValues.Field(i).Field(0).Float()
			op_time_second := float64(op_time_microsecond / 1e6)

			topTimeSecondsTotal.WithLabelValues(metric_type, database, collection).Set(op_time_second)
			topCountTotal.WithLabelValues(metric_type, database, collection).Set(op_count)
		}
	}

	topTimeSecondsTotal.Collect(ch)
	topCountTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (tops TopStatsMap) Describe(ch chan<- *prometheus.Desc) {
	topTimeSecondsTotal.Describe(ch)
	topCountTotal.Describe(ch)
}
