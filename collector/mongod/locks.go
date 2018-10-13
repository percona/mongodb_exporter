// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	locksTimeLockedGlobalMicrosecondsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "locks_time_locked_global_microseconds_total",
		Help:      "amount of time in microseconds that any database has held the global lock",
	}, []string{"type", "database"})
)
var (
	locksTimeLockedLocalMicrosecondsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "locks_time_locked_local_microseconds_total",
		Help:      "amount of time in microseconds that any database has held the local lock",
	}, []string{"type", "database"})
)
var (
	locksTimeAcquiringGlobalMicrosecondsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "locks_time_acquiring_global_microseconds_total",
		Help:      "amount of time in microseconds that any database has spent waiting for the global lock",
	}, []string{"type", "database"})
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

		locksTimeLockedGlobalMicrosecondsTotal.WithLabelValues("read", key).Set(locks.TimeLockedMicros.Read)
		locksTimeLockedGlobalMicrosecondsTotal.WithLabelValues("write", key).Set(locks.TimeLockedMicros.Write)

		locksTimeLockedLocalMicrosecondsTotal.WithLabelValues("read", key).Set(locks.TimeLockedMicros.ReadLower)
		locksTimeLockedLocalMicrosecondsTotal.WithLabelValues("write", key).Set(locks.TimeLockedMicros.WriteLower)

		locksTimeAcquiringGlobalMicrosecondsTotal.WithLabelValues("read", key).Set(locks.TimeAcquiringMicros.ReadLower)
		locksTimeAcquiringGlobalMicrosecondsTotal.WithLabelValues("write", key).Set(locks.TimeAcquiringMicros.WriteLower)
	}

	locksTimeLockedGlobalMicrosecondsTotal.Collect(ch)
	locksTimeLockedLocalMicrosecondsTotal.Collect(ch)
	locksTimeAcquiringGlobalMicrosecondsTotal.Collect(ch)
}

// Describe describes the metrics for prometheus
func (locks LockStatsMap) Describe(ch chan<- *prometheus.Desc) {
	locksTimeLockedGlobalMicrosecondsTotal.Describe(ch)
	locksTimeLockedLocalMicrosecondsTotal.Describe(ch)
	locksTimeAcquiringGlobalMicrosecondsTotal.Describe(ch)
}
