package collector_mongod

import (
	"testing"
)

func Test_ConnectionsCollectData(t *testing.T) {
	stats := &ConnectionStats{
		Current:      1,
		Available:    2,
		TotalCreated: 3,
	}

	stats.Export()
}
