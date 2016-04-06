package collector_mongod

import (
	"testing"
)

func Test_NetworkCollectData(t *testing.T) {
	stats := &NetworkStats{}

	stats.Export()
}
