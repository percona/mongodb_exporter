package collector_mongod

import (
	"testing"
)

func Test_GlobalLockCollectData(t *testing.T) {
	stats := &GlobalLockStats{
		CurrentQueue:  &QueueStats{},
		ActiveClients: &ClientStats{},
	}

	stats.Export()
}
