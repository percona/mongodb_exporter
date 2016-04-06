package collector_mongos

import (
	"testing"
)

func Test_ExtraInfoCollectData(t *testing.T) {
	stats := &ExtraInfo{}

	stats.Export()
}
