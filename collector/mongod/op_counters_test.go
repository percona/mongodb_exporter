package collector_mongod

import (
	"testing"
)

func Test_OpCountersCollectData(t *testing.T) {
	stats := &OpcountersStats{}

	stats.Export()
}
