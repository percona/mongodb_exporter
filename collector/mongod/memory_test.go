package collector_mongod

import (
	"testing"
)

func Test_MemoryCollectData(t *testing.T) {
	stats := &MemStats{}

	stats.Export()
}
