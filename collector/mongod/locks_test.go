package collector_mongod

import (
	"testing"
)

func Test_LocksCollectData(t *testing.T) {
	stats := &LockStatsMap{
		".": LockStats{
			TimeLockedMicros:    ReadWriteLockTimes{},
			TimeAcquiringMicros: ReadWriteLockTimes{},
		},
	}

	stats.Export()
}
