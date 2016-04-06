package collector_mongos

import (
	"testing"
)

func Test_NetworkCollectData(t *testing.T) {
	stats := &NetworkStats{}

	stats.Export()
}
