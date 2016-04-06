package collector_mongod

import (
	"testing"
)

func Test_DurabilityCollectData(t *testing.T) {
	stats := &DurStats{
		TimeMs: DurTiming{},
	}

	stats.Export()
}
