package collector_mongod

import (
	"testing"
)

func Test_MetricsCollectData(t *testing.T) {
	stats := &MetricsStats{
		Document: &DocumentStats{},
		GetLastError: &GetLastErrorStats{
			Wtime: &BenchmarkStats{},
		},
		Operation:     &OperationStats{},
		QueryExecutor: &QueryExecutorStats{},
		Record:        &RecordStats{},
		Repl: &ReplStats{
			Apply: &ApplyStats{
				Batches: &BenchmarkStats{},
			},
			Buffer: &BufferStats{},
			Network: &MetricsNetworkStats{
				GetMores: &BenchmarkStats{},
			},
			PreloadStats: &PreloadStats{
				Docs:    &BenchmarkStats{},
				Indexes: &BenchmarkStats{},
			},
		},
		Storage: &StorageStats{},
	}

	stats.Export()
}
