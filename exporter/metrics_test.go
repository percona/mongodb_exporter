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

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(staticCollector(metrics))

	gatheredMetrics, err := reg.Gather()
	assert.NoError(t, err, "metrics with the same name and labels must not be exported")

	metricsByName := make(map[string]*dto.MetricFamily, len(gatheredMetrics))
	for _, metric := range gatheredMetrics {
		metricsByName[metric.GetName()] = metric
	}

	assert.NotContains(t, metricsByName, "mongodb_ss_metrics_query_multiPlanner_histograms_sbeMicros_lowerBound")

	bucketCounts, ok := metricsByName["mongodb_ss_metrics_query_multiPlanner_histograms_sbeMicros_count"]
	if !assert.True(t, ok) {
		return
	}

	bucketCountMetrics := bucketCounts.GetMetric()
	if !assert.Len(t, bucketCountMetrics, 2) {
		return
	}

	valuesByBound := make(map[string]float64, len(bucketCountMetrics))
	for _, metric := range bucketCountMetrics {
		labels := make(map[string]string, len(metric.GetLabel()))
		for _, label := range metric.GetLabel() {
			labels[label.GetName()] = label.GetValue()
		}
		valuesByBound[labels["lower_bound"]] = metric.GetCounter().GetValue()
	}

	assert.Equal(t, map[string]float64{
		"0":    3,
		"1024": 7,
	}, valuesByBound)
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

// ethtool reports several counters per queue whose names differ only in leading spaces,
// which used to produce one metric name with two different help strings.
func TestKeysCollidingAfterSanitizationStayDistinct(t *testing.T) {
	t.Parallel()

	metrics := makeMetrics("systemMetrics.ethtool", bson.M{
		"ens192": bson.M{
			"     giant hdr": int64(11),
			"  giant hdr":    int64(22),
			"tx queue":       int64(33),
		},
	}, nil, false)

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(staticCollector(metrics))

	gatheredMetrics, err := reg.Gather()
	require.NoError(t, err, "colliding keys must not break the whole scrape")

	metricsByName := make(map[string]*dto.MetricFamily, len(gatheredMetrics))
	for _, metric := range gatheredMetrics {
		metricsByName[metric.GetName()] = metric
	}

	assert.Contains(t, metricsByName, "mongodb_sys_ethtool_ens192_tx_queue")

	collided, ok := metricsByName["mongodb_sys_ethtool_ens192_giant_hdr"]
	if !assert.True(t, ok) {
		return
	}

	valuesByIndex := make(map[string]float64, len(collided.GetMetric()))
	for _, metric := range collided.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == collisionLabel {
				valuesByIndex[label.GetValue()] = metric.GetUntyped().GetValue()
			}
		}
	}

	// Sorted by the raw key, so the one with more leading spaces comes first.
	assert.Equal(t, map[string]float64{
		"0": 11,
		"1": 22,
	}, valuesByIndex)
}

func TestMetricHelpNormalizesSpaces(t *testing.T) {
	t.Parallel()

	assert.Equal(t, metricHelp("systemMetrics.ethtool.ens192.", "  giant hdr"),
		metricHelp("systemMetrics.ethtool.ens192.", "     giant hdr"))

	// Help strings without redundant whitespace must be left untouched.
	assert.Equal(t, "local.oplog.rs.stats.wiredTiger.btree.fixed-record size",
		metricHelp("local.oplog.rs.stats.wiredTiger.btree.", "fixed-record size"))
	assert.Equal(t, "serverStatus.connections", metricHelp("serverStatus.connections.", "current"))
}

func TestCollidingKeyIndexes(t *testing.T) {
	t.Parallel()

	assert.Nil(t, collidingKeyIndexes("systemMetrics.", bson.M{"a b": int64(1), "c d": int64(2)}))
	assert.Nil(t, collidingKeyIndexes("systemMetrics.", bson.M{"plain": int64(1)}))
	assert.Nil(t, collidingKeyIndexes("systemMetrics.", bson.M{"a_b": int64(1), "c_d": int64(2)}))
	assert.Equal(t, map[string]int{"a b": 0, "a_b": 1}, collidingKeyIndexes("systemMetrics.", bson.M{
		"a b":   int64(1),
		"a_b":   int64(2),
		"other": int64(3),
	}))

	// prometheusize also drops a trailing underscore and collapses underscore runs, so keys
	// differing only there end up sharing one metric name.
	assert.Equal(t, map[string]int{"giant hdr": 0, "giant hdr#": 1},
		collidingKeyIndexes("systemMetrics.ethtool.ens192.", bson.M{
			"giant hdr":  int64(1),
			"giant hdr#": int64(2),
		}))
	assert.Equal(t, map[string]int{"a__b": 0, "a_b": 1},
		collidingKeyIndexes("systemMetrics.", bson.M{"a__b": int64(1), "a_b": int64(2)}))
}

func TestIsUnambiguousKey(t *testing.T) {
	t.Parallel()

	for _, k := range []string{"plain", "a_b", "Tx0_queue_1"} {
		assert.True(t, isUnambiguousKey(k), k)
	}

	// Everything prometheusize would rewrite has to be reported as ambiguous.
	for _, k := range []string{"", "_", "a b", "a-b", "a__b", "_a", "a_", "Tx Queue#", "říká"} {
		assert.False(t, isUnambiguousKey(k), k)
	}
}

func TestAsMetricMapHandlesBSONM(t *testing.T) {
	t.Parallel()

	bucket, ok := asMetricMap(bson.M{"lowerBound": int64(1024), "count": int64(7)})

	assert.True(t, ok)
	assert.Equal(t, int64(1024), bucket["lowerBound"])
	assert.Equal(t, int64(7), bucket["count"])
}
