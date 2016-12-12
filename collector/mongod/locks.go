package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	locksTimeLockedGlobalMicrosecondsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "locks_time_locked_global_microseconds_total"),
		"amount of time in microseconds that any database has held the global lock",
	  []string{"type", "database"}, nil)
)
var (
	locksTimeLockedLocalMicrosecondsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "locks_time_locked_local_microseconds_total"),
		"amount of time in microseconds that any database has held the local lock",
	  []string{"type", "database"}, nil)
)
var (
	locksTimeAcquiringGlobalMicrosecondsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "locks_time_acquiring_global_microseconds_total"),
		"amount of time in microseconds that any database has spent waiting for the global lock",
	  []string{"type", "database"}, nil)
)

// LockStatsMap is a map of lock stats
type LockStatsMap map[string]LockStats

// ReadWriteLockTimes information about the lock
type ReadWriteLockTimes struct {
	Read       float64 `bson:"R"`
	Write      float64 `bson:"W"`
	ReadLower  float64 `bson:"r"`
	WriteLower float64 `bson:"w"`
}

// LockStats lock stats
type LockStats struct {
	TimeLockedMicros    ReadWriteLockTimes `bson:"timeLockedMicros"`
	TimeAcquiringMicros ReadWriteLockTimes `bson:"timeAcquiringMicros"`
}

// Export exports the data to prometheus.
func (locks LockStatsMap) Export(ch chan<- prometheus.Metric) {
	for key, locks := range locks {
		if key == "." {
			key = "dot"
		}

		ch <- prometheus.MustNewConstMetric(locksTimeLockedGlobalMicrosecondsTotal, prometheus.CounterValue, locks.TimeLockedMicros.Read, "read", key)
		ch <- prometheus.MustNewConstMetric(locksTimeLockedGlobalMicrosecondsTotal, prometheus.CounterValue, locks.TimeLockedMicros.Write, "write", key)

		ch <- prometheus.MustNewConstMetric(locksTimeLockedLocalMicrosecondsTotal, prometheus.CounterValue, locks.TimeLockedMicros.ReadLower, "read", key)
		ch <- prometheus.MustNewConstMetric(locksTimeLockedLocalMicrosecondsTotal, prometheus.CounterValue, locks.TimeLockedMicros.WriteLower, "write", key)

		ch <- prometheus.MustNewConstMetric(locksTimeAcquiringGlobalMicrosecondsTotal, prometheus.CounterValue, locks.TimeAcquiringMicros.ReadLower, "read", key)
		ch <- prometheus.MustNewConstMetric(locksTimeAcquiringGlobalMicrosecondsTotal, prometheus.CounterValue, locks.TimeAcquiringMicros.WriteLower, "write", key)
	}
}

// Describe describes the metrics for prometheus
func (locks LockStatsMap) Describe(ch chan<- *prometheus.Desc) {
	ch <- locksTimeLockedGlobalMicrosecondsTotal
	ch <- locksTimeLockedLocalMicrosecondsTotal
	ch <- locksTimeAcquiringGlobalMicrosecondsTotal
}
