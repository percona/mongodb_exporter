// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"testing"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type staticCollector []prometheus.Metric

func (c staticCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c {
		ch <- metric.Desc()
	}
}

func (c staticCollector) Collect(ch chan<- prometheus.Metric) {
	for _, metric := range c {
		ch <- metric
	}
}

// Test metric renaming and labeling.
func TestMetricName(t *testing.T) {
	tcs := []struct {
		prefix     string
		name       string
		wantMetric string
		wantLabel  string
	}{
		{
			prefix:     "serverStatus.metrics.commands.saslStart.",
			name:       "total",
			wantMetric: "mongodb_ss_metrics_commands_saslStart_total",
		},
		{
			prefix:     "serverStatus.metrics.commands._configsvrShardCollection.",
			name:       "failed",
			wantMetric: "mongodb_ss_metrics_commands_configsvrShardCollection_failed",
		},
		{
			prefix:     "serverStatus.wiredTiger.lock.",
			name:       "metadata lock acquisitions",
			wantMetric: "mongodb_ss_wt_lock_metadata_lock_acquisitions",
		},
		{
			prefix:     "serverStatus.wiredTiger.perf.",
			name:       "file system write latency histogram (bucket 5) - 500-999ms",
			wantMetric: "mongodb_ss_wt_perf",
			wantLabel:  "perf_bucket",
		},
		{
			prefix:     "serverStatus.wiredTiger.transaction.",
			name:       "rollback to stable updates removed from lookaside",
			wantMetric: "mongodb_ss_wt_txn_rollback_to_stable_updates_removed_from_lookaside",
		},
	}

	for _, tc := range tcs {
		metric, label := nameAndLabel(tc.prefix, tc.name)
		assert.Equal(t, tc.wantMetric, metric, tc.prefix+tc.name)
		assert.Equal(t, tc.wantLabel, label, tc.prefix+tc.name)
	}
}

func TestPrometeusize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in:   "serverStatus.wiredTiger.transaction.transaction checkpoint most recent time (msecs)",
			want: "mongodb_ss_wt_txn_transaction_checkpoint_most_recent_time_msecs",
		},
		{
			in:   "serverStatus.wiredTiger.thread-yield.page acquire time sleeping (usecs)",
			want: "mongodb_ss_wt_thread_yield_page_acquire_time_sleeping_usecs",
		},
		{
			in:   "serverStatus.opLatencies.reads.latency",
			want: "mongodb_ss_opLatencies_reads_latency",
		},
		{
			in:   "replSetGetStatus.optimes.lastCommittedOpTime.t",
			want: "mongodb_rs_optimes_lastCommittedOpTime_t",
		},
		{
			in:   "systemMetrics.memory.Active_kb",
			want: "mongodb_sys_memory_Active_kb",
		},
		{
			in:   "local.oplog.rs.stats.wiredTiger.block-manager.checkpoint size",
			want: "mongodb_oplog_stats_wt_block_manager_checkpoint_size",
		},
		{
			in:   "local.oplog.rs.stats.storageSize",
			want: "mongodb_oplog_stats_storageSize",
		},
		{
			in:   "collstats_storage.wiredTiger.xxx",
			want: "mongodb_collstats_storage_wt_xxx",
		},

		{
			in:   "collstats_storage.indexDetails.xxx",
			want: "mongodb_collstats_storage_idx_xxx",
		},
		{
			in:   "collStats.storageStats.xxx",
			want: "mongodb_collstats_storage_xxx",
		},
		{
			in:   "collStats.latencyStats.xxx",
			want: "mongodb_collstats_latency_xxx",
		},
	}

	for _, test := range tests {
		got := prometheusize(test.in)
		assert.Equal(t, test.want, got)
	}
}

// Test supported value types conversion.
func TestMakeRawMetric(t *testing.T) {
	prefix := "serverStatus.transactions."
	name := "retriedCommandsCount"
	testCases := []struct {
		value   any
		wantVal *float64
	}{
		{value: true, wantVal: pointer.ToFloat64(1)},
		{value: false, wantVal: pointer.ToFloat64(0)},
		{value: int32(1), wantVal: pointer.ToFloat64(1)},
		{value: int64(2), wantVal: pointer.ToFloat64(2)},
		{value: float32(1.23), wantVal: new(float64(float32(1.23)))},
		{value: float64(1.23), wantVal: new(1.23)},
		{value: primitive.A{}, wantVal: nil},
		{value: primitive.Timestamp{T: 123, I: 456}, wantVal: pointer.ToFloat64(123)},
		{value: "zapp", wantVal: nil},
		{value: []byte{}, wantVal: nil},
		{value: time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC), wantVal: nil},
	}

	ln := make([]string, 0) // needs pre-allocation to accomplish pre-allocation for labels
	lv := make([]string, 0)

	fqName := prometheusize(prefix + name)
	help := metricHelp(prefix, name)

	for _, tc := range testCases {
		var want *rawMetric
		if tc.wantVal != nil {
			want = &rawMetric{
				fqName: fqName,
				help:   help,
				ln:     ln,
				lv:     lv,
				val:    *tc.wantVal,
				vt:     prometheus.CounterValue,
			}
		}

		m, err := makeRawMetric(prefix, name, tc.value, nil)

		assert.NoError(t, err)
		assert.Equal(t, want, m)
	}
}

func TestRawToCompatibleRawMetric(t *testing.T) {
	testCases := []struct {
		in   *rawMetric
		want *rawMetric
	}{
		{
			in: &rawMetric{
				fqName: "mongodb_ss_opLatencies_commands_latency",
				val:    float64(1),
				vt:     prometheus.UntypedValue,
			},
			want: &rawMetric{
				fqName: "mongodb_ss_opLatencies_latency",
				help:   "mongodb_ss_opLatencies_latency",
				ln:     []string{"op_type"},
				lv:     []string{"commands"},
				val:    1,
				vt:     3,
			},
		},
		{
			in: &rawMetric{
				fqName: "mongodb_ss_opLatencies_commands_ops",
				val:    float64(1),
				vt:     prometheus.UntypedValue,
			},
			want: &rawMetric{
				fqName: "mongodb_ss_opLatencies_ops",
				help:   "mongodb_ss_opLatencies_ops",
				ln:     []string{"op_type"},
				lv:     []string{"commands"},
				val:    1,
				vt:     3,
			},
		},
	}

	for _, tc := range testCases {
		m := metricRenameAndLabel(tc.in, specialConversions)
		assert.Equal(t, m[0], tc.want)
	}
}

// Histogram buckets must not trigger "was collected before with the same name and label values".
func TestHistogramMetricsDoNotCollide(t *testing.T) {
	t.Parallel()

	metrics := makeMetricsWithHistograms("serverStatus.metrics.query.multiPlanner.histograms", bson.M{
		"sbeMicros": primitive.A{
			bson.M{"lowerBound": int64(0), "count": int64(3)},
			bson.M{"lowerBound": int64(1024), "count": int64(7)},
		},
	}, nil, true, true)

	metricsByName := gatherMetrics(t, metrics)

	assert.NotContains(t, metricsByName, "mongodb_ss_metrics_query_multiPlanner_histograms_sbeMicros_lowerBound")

	bucketCounts, ok := metricsByName["mongodb_ss_metrics_query_multiPlanner_histograms_sbeMicros_count"]
	require.True(t, ok)
	assert.Len(t, bucketCounts.GetMetric(), 2)

	assert.Equal(t, map[string]float64{
		"0":    3,
		"1024": 7,
	}, countsByLowerBound(bucketCounts))
}

func TestHistogramMetricsAreSkippedByDefault(t *testing.T) {
	t.Parallel()

	metrics := makeMetrics("serverStatus.metrics.query.multiPlanner", bson.M{
		"histograms": bson.M{
			"sbeMicros": primitive.A{
				bson.M{"lowerBound": int64(0), "count": int64(3)},
				bson.M{"lowerBound": int64(1024), "count": int64(7)},
			},
		},
	}, nil, true)

	assert.Empty(t, metrics)

	metrics = makeMetrics("serverStatus.metrics.query.multiPlanner.histograms", bson.M{
		"sbeMicros": primitive.A{
			bson.M{"lowerBound": int64(0), "count": int64(3)},
			bson.M{"lowerBound": int64(1024), "count": int64(7)},
		},
	}, nil, true)

	assert.Empty(t, metrics)
}

// Only the buckets are gated by includeHistograms. A node named "histogram" that holds
// something else must be collected like any other node.
func TestNonBucketHistogramNodeIsNotSkipped(t *testing.T) {
	t.Parallel()

	metrics := makeMetrics("serverStatus.someFeature", bson.M{
		"histogram": bson.M{
			"enabled":   int64(1),
			"sizeBytes": int64(42),
		},
	}, nil, false)

	assert.ElementsMatch(t, []string{
		"mongodb_ss_someFeature_histogram_enabled",
		"mongodb_ss_someFeature_histogram_sizeBytes",
	}, gatheredMetricNames(t, metrics))
}

// serverStatus.opLatencies exposes its buckets under "histogram" with a "micros" boundary,
// which used to be flattened into one metric per bucket sharing the same name and labels.
func TestOpLatenciesHistogramMetricsDoNotCollide(t *testing.T) {
	t.Parallel()

	metrics := makeMetricsWithHistograms("serverStatus", bson.M{
		"opLatencies": bson.M{
			"reads": bson.M{
				"latency": int64(120),
				"ops":     int64(4),
				"histogram": primitive.A{
					bson.M{"micros": int64(1), "count": int64(3)},
					bson.M{"micros": int64(2048), "count": int64(7)},
				},
			},
		},
	}, nil, false, true)

	metricsByName := gatherMetrics(t, metrics)

	assert.NotContains(t, metricsByName, "mongodb_ss_opLatencies_reads_histogram_micros")

	bucketCounts, ok := metricsByName["mongodb_ss_opLatencies_reads_histogram_count"]
	require.True(t, ok)
	assert.Len(t, bucketCounts.GetMetric(), 2)

	assert.Equal(t, map[string]float64{
		"1":    3,
		"2048": 7,
	}, countsByLowerBound(bucketCounts))
}

// The bucket metrics carry the "mongodb_ss_opLatencies" prefix that specialConversions and the
// v1 conversions match on, and compatibleMode exports a second series set from the same raw
// metric. Neither may rename the buckets onto the op_type series.
func TestOpLatenciesHistogramMetricsInCompatibleMode(t *testing.T) {
	t.Parallel()

	metrics := makeMetricsWithHistograms("serverStatus", bson.M{
		"opLatencies": bson.M{
			"reads": bson.M{
				"latency": int64(120),
				"ops":     int64(4),
				"histogram": primitive.A{
					bson.M{"micros": int64(1), "count": int64(3)},
					bson.M{"micros": int64(2048), "count": int64(7)},
				},
			},
		},
	}, nil, true, true)

	metricsByName := gatherMetrics(t, metrics)

	bucketCounts, ok := metricsByName["mongodb_ss_opLatencies_reads_histogram_count"]
	require.True(t, ok)
	assert.Equal(t, map[string]float64{
		"1":    3,
		"2048": 7,
	}, countsByLowerBound(bucketCounts))

	// The buckets get no op_type series and no v1 compatible twin of their own.
	assert.Empty(t, labelValues(bucketCounts, "op_type"))
	assert.Equal(t, []string{"reads"}, labelValues(metricsByName["mongodb_ss_opLatencies_latency"], "op_type"))
	assert.Len(t, metricsByName["mongodb_mongod_op_latencies_latency_total"].GetMetric(), 1)
}

func TestOpLatenciesHistogramMetricsAreSkippedByDefault(t *testing.T) {
	t.Parallel()

	metrics := makeMetrics("serverStatus.opLatencies.reads", bson.M{
		"latency": int64(120),
		"histogram": primitive.A{
			bson.M{"micros": int64(1), "count": int64(3)},
			bson.M{"micros": int64(2048), "count": int64(7)},
		},
	}, nil, false)

	// The latency metric is renamed and labeled by specialConversions, the buckets are dropped.
	assert.Equal(t, []string{"mongodb_ss_opLatencies_latency"}, gatheredMetricNames(t, metrics))
}

func TestAsMetricMapHandlesBSONM(t *testing.T) {
	t.Parallel()

	bucket, ok := asMetricMap(bson.M{"lowerBound": int64(1024), "count": int64(7)})

	assert.True(t, ok)
	assert.Equal(t, int64(1024), bucket["lowerBound"])
	assert.Equal(t, int64(7), bucket["count"])
}

// MongoDB 8.3 added a histogram to serverStatus().opLatencies. Its buckets are keyed by
// "micros" rather than "lowerBound", and the field is spelled "histogram" rather than
// "histograms", so neither the bucket shape nor the path was recognised: every bucket
// produced the same series and the registry rejected all but the first, one error per
// bucket per scrape.
func TestOpLatenciesHistogramBucketsDoNotCollide(t *testing.T) {
	t.Parallel()

	metrics := makeMetricsWithHistograms("serverStatus.opLatencies.commands", bson.M{
		"histogram": primitive.A{
			bson.M{"micros": int64(8), "count": int64(3)},
			bson.M{"micros": int64(64), "count": int64(7)},
			bson.M{"micros": int64(512), "count": int64(11)},
		},
		"latency": int64(1234),
		"ops":     int64(5),
	}, nil, true, true)

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(staticCollector(metrics))

	gatheredMetrics, err := reg.Gather()
	require.NoError(t, err, "metrics with the same name and labels must not be exported")

	metricsByName := make(map[string]*dto.MetricFamily, len(gatheredMetrics))
	for _, metric := range gatheredMetrics {
		metricsByName[metric.GetName()] = metric
	}

	// The bucket bound is a label, not a measurement of its own.
	assert.NotContains(t, metricsByName, "mongodb_ss_opLatencies_commands_histogram_micros")

	bucketCounts, ok := metricsByName["mongodb_ss_opLatencies_commands_histogram_count"]
	if !assert.True(t, ok) {
		return
	}

	valuesByBound := make(map[string]float64, len(bucketCounts.GetMetric()))
	for _, metric := range bucketCounts.GetMetric() {
		valuesByBound[metricLabels(metric)["micros"]] = metric.GetCounter().GetValue()
	}

	assert.Equal(t, map[string]float64{
		"8":   3,
		"64":  7,
		"512": 11,
	}, valuesByBound)

	// Siblings of the histogram are still reported. nodeToPDMetrics turns the op name into a
	// label for these, which is why they are not scoped by "commands" the way the bucket
	// counts are.
	assert.Contains(t, metricsByName, "mongodb_ss_opLatencies_latency")
	assert.Contains(t, metricsByName, "mongodb_ss_opLatencies_ops")
}

// The singular spelling is gated like the plural one, so enabling histograms stays the one
// switch that decides whether bucket series are exported.
func TestOpLatenciesHistogramSkippedByDefault(t *testing.T) {
	t.Parallel()

	metrics := makeMetrics("serverStatus.opLatencies.commands", bson.M{
		"histogram": primitive.A{
			bson.M{"micros": int64(8), "count": int64(3)},
		},
		"ops": int64(5),
	}, nil, true)

	for _, metric := range metrics {
		assert.NotContains(t, metric.Desc().String(), "histogram")
	}
}

// MongoDB 9.0 added queue wait time histograms to serverStatus().queues and to the
// wiredTiger.concurrentTransactions section of getDiagnosticData. They are arrays of
// lowerBound buckets on a path with no "histogram" in it, so every bucket produced the
// same series and the registry rejected all but the first.
func TestQueueWaitTimeHistogram(t *testing.T) {
	t.Parallel()

	const prefix = "serverStatus.queues.execution.read.normalPriority"
	m := bson.M{
		"queueWaitTimeMicros": primitive.A{
			bson.M{"lowerBound": int64(0), "count": int64(291)},
			bson.M{"lowerBound": int64(1), "count": int64(2)},
			bson.M{"lowerBound": int64(10), "count": int64(0)},
		},
		"processing": int64(1),
	}

	t.Run("buckets do not collide", func(t *testing.T) {
		t.Parallel()

		reg := prometheus.NewPedanticRegistry()
		reg.MustRegister(staticCollector(makeMetricsWithHistograms(prefix, m, nil, true, true)))

		gatheredMetrics, err := reg.Gather()
		require.NoError(t, err, "metrics with the same name and labels must not be exported")

		metricsByName := make(map[string]*dto.MetricFamily, len(gatheredMetrics))
		for _, metric := range gatheredMetrics {
			metricsByName[metric.GetName()] = metric
		}

		assert.NotContains(t, metricsByName, "mongodb_ss_wt_concurrentTransactions_read_normalPriority_queueWaitTimeMicros_lowerBound")

		bucketCounts, ok := metricsByName["mongodb_ss_wt_concurrentTransactions_read_normalPriority_queueWaitTimeMicros_count"]
		if !assert.True(t, ok) {
			return
		}

		valuesByBound := make(map[string]float64, len(bucketCounts.GetMetric()))
		for _, metric := range bucketCounts.GetMetric() {
			valuesByBound[metricLabels(metric)["lower_bound"]] = metric.GetCounter().GetValue()
		}
		assert.Equal(t, map[string]float64{"0": 291, "1": 2, "10": 0}, valuesByBound)

		assert.Contains(t, metricsByName, "mongodb_ss_wt_concurrentTransactions_read_normalPriority_processing")
	})

	t.Run("skipped by default", func(t *testing.T) {
		t.Parallel()

		metrics := makeMetrics(prefix, m, nil, true)
		require.NotEmpty(t, metrics)
		for _, metric := range metrics {
			assert.NotContains(t, metric.Desc().String(), "queueWaitTimeMicros")
		}
	})
}
