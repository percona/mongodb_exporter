package collector_mongos

import (
	"testing"
)

func Test_MetricsCollectData(t *testing.T) {
	stats := &MetricsStats{
		GetLastError: &GetLastErrorStats{
			Wtime: &BenchmarkStats{},
		},
	}

	stats.Export()
}
