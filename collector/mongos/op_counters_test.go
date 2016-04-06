package collector_mongos

import (
	"testing"
)

func Test_OpCountersCollectData(t *testing.T) {
	stats := &OpcountersStats{}

	stats.Export()
}
