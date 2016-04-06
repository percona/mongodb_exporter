package collector_mongod

import (
	"testing"
)

func Test_BackgroundFlushingCollectData(t *testing.T) {
	stats := &FlushStats{
		Flushes:   1,
		TotalMs:   2,
		AverageMs: 3,
		LastMs:    4,
	}

	stats.Export()
}
