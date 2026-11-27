// mongodb_exporter
// Copyright (C) 2026 Percona LLC
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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

var testCounterFamilies = []string{
	"mongodb_ss_opcounters",
	"mongodb_ss_opcountersRepl",
	"mongodb_ss_asserts",
	"mongodb_ss_network_bytesIn",
	"mongodb_ss_network_bytesOut",
	"mongodb_ss_network_numRequests",
	"mongodb_ss_opLatencies_latency",
	"mongodb_ss_opLatencies_ops",
	"mongodb_ss_opLatencies_reads_histogram_count",
	"mongodb_ss_locks_Global_acquireCount_r",
	"mongodb_ss_locks_acquireCount",
	"mongodb_ss_metrics_document",
	"mongodb_ss_metrics_commands_find_total",
	"mongodb_ss_metrics_cursor_timedOut",
	"mongodb_ss_metrics_query_multiPlanner_histograms_classicMicros_count",
	"mongodb_ss_extra_info_page_faults",
	"mongodb_ss_transactions_totalCommitted",
	"mongodb_ss_electionMetrics_stepUpCmd_called",
	"mongodb_ss_shardingStatistics_catalogCache_countStaleConfigErrors",
	"mongodb_ss_wt_concurrentTransactions_read_normalPriority_queueWaitTimeMicros_count",
	"mongodb_ss_wt_concurrentTransactions_read_normalPriority_totalTimeQueuedMicros",
	"mongodb_ss_wt_cache_pages_read_into_cache",
	"mongodb_ss_wt_txn_transaction_checkpoint_total_time_msecs",
	"mongodb_ss_wt_checkpoint_total_time_msecs",
	"mongodb_ss_wt_session_table_create_successful_calls",
	"mongodb_oplog_stats_wt_transaction_update_conflicts",
	"mongodb_collstats_storageStats_wiredTiger_cursor_insert_calls",
	"mongodb_collstats_storageStats_indexDetails_index_name_cache_bytes_read_into_cache",
	"mongodb_sys_cpu_user_ms",
	"mongodb_sys_disks_sda_reads",
	"mongodb_sys_netstat_Tcp_InSegs",
	"mongodb_sys_vmstat_pgfault",
	"mongodb_collstats_latencyStats_reads_ops",
	"mongodb_indexstats_accesses_ops",
	"mongodb_top_insert_count",
	"mongodb_op_counters_total",
	"mongodb_mongod_metrics_document_total",
	"mongodb_mongod_metrics_repl_apply_batches_total_milliseconds",
	"mongodb_mongod_wiredtiger_cache_evicted_total",
	"mongodb_mongod_locks_time_acquiring_global_microseconds_total",
}

var testGaugeFamilies = []string{
	// #803 and #1388
	"mongodb_collstats_storageStats_count",
	"mongodb_ss_wt_session_open_session_count",
	"mongodb_ss_wt_cursor_open_cursor_count",
	"mongodb_rs_votingMembersCount",
	"mongodb_ss_logicalSessionRecordCache_activeSessionsCount",
	"mongodb_ss_metrics_repl_buffer_count",
	"mongodb_ss_connections",
	"mongodb_ss_mem_resident",
	"mongodb_ss_globalLock_currentQueue",
	"mongodb_ss_metrics_cursor_open",
	"mongodb_ss_transactions_currentOpen",
	"mongodb_ss_uptime",
	"mongodb_sys_memory_MemFree_kb",
	"mongodb_sys_netstat_Tcp_CurrEstab",
	"mongodb_sys_disks_sda_io_in_progress",
	"mongodb_sys_vmstat_nr_mlock",
	// #1229
	"mongodb_collstats_storageStats_indexSizes",
	"mongodb_ss_wt_cache_bytes_currently_in_the_cache",
	"mongodb_ss_wt_cache_maximum_bytes_configured",
	"mongodb_ss_wt_txn_transaction_checkpoint_currently_running",
	"mongodb_ss_wt_txn_prepared_transactions_currently_active",
	"mongodb_ss_wt_historyStorageStats_btree_number_of_key_value_pairs",
	"mongodb_ss_wt_concurrentTransactions_available",
	"mongodb_oplog_stats_wt_btree_fixed_record_size",
	"mongodb_oplog_stats_count",
	"mongodb_rs_members_health",
	"mongodb_rs_cfg_protocolVersion",
	"mongodb_dbstats_dataSize",
	"mongodb_profile_slow_query_count",
	"mongodb_mongod_db_collections_total",
	"mongodb_mongod_wiredtiger_session_open_sessions_total",
	"mongodb_mongod_metrics_cursor_open",
	// Fields no rule covers
	"mongodb_ss_catalogStats_collections",
	"mongodb_ss_some_future_field",
}

func TestMetricType(t *testing.T) {
	t.Parallel()

	for _, name := range testCounterFamilies {
		assert.Equal(t, prometheus.CounterValue, metricType(name), name)
	}

	for _, name := range testGaugeFamilies {
		assert.Equal(t, prometheus.GaugeValue, metricType(name), name)
	}
}

func TestWiredTigerStatLists(t *testing.T) {
	t.Parallel()

	counters := make(map[string]bool, len(wiredTigerCounters))
	for _, stat := range wiredTigerCounters {
		counters[stat] = true
	}

	for _, stat := range wiredTigerGauges {
		assert.False(t, counters[stat], "%q is listed as a counter and a gauge", stat)
	}

	assert.Equal(t, "cache_bytes_currently_in_the_cache", wiredTigerStatKey("cache: bytes currently in the cache"))
	assert.Equal(t, "transaction_transaction_checkpoint_total_time_msecs",
		wiredTigerStatKey("transaction: transaction checkpoint total time (msecs)"))
}

// Every series of a family must have the same type, whatever the field it comes from is
// called. An index whose name ends in "count" used to make its series a counter while the
// other indexes of the collection were untyped, see #1229.
func TestIndexNamedCountHasTheSameTypeAsTheOtherIndexes(t *testing.T) {
	t.Parallel()

	m := bson.M{
		"storageStats": bson.M{
			"count": int32(2),
			"indexSizes": bson.M{
				"_id_":                  int32(4096),
				"ix_task_context.count": int32(20480),
				"amount":                int32(20480),
			},
		},
	}

	labels := map[string]string{"database": "db", "collection": "c"}
	metrics := makeMetrics("collstats", m, labels, false)
	metrics = append(metrics, makeMetrics("config.transactions.stats", m, labels, false)...)

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(staticCollector(metrics))

	families, err := reg.Gather()
	require.NoError(t, err)

	types := map[string]dto.MetricType{}
	for _, family := range families {
		types[family.GetName()] = family.GetType()
	}

	assert.Equal(t, dto.MetricType_GAUGE, types["mongodb_collstats_storageStats_indexSizes"])
	assert.Equal(t, dto.MetricType_GAUGE, types["mongodb_collstats_storageStats_count"])
	assert.Equal(t, dto.MetricType_GAUGE, types["mongodb_config_transactions_stats_storageStats_indexSizes"])
}

// The whole getDiagnosticData document, including the metric names of mongodb_exporter v0.11,
// must be exported without type conflicts and without untyped metrics.
func TestDiagnosticDataHasNoUntypedMetrics(t *testing.T) {
	t.Parallel()

	buf, err := os.ReadFile(filepath.Join("testdata", "get_diagnostic_data.json"))
	require.NoError(t, err)

	var m bson.M
	require.NoError(t, json.Unmarshal(buf, &m))

	metrics := makeMetricsWithHistograms("", m, nil, true, true)
	metrics = append(metrics, locksMetrics(nil, m)...)

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(staticCollector(metrics))

	families, err := reg.Gather()
	require.NoError(t, err)
	require.NotEmpty(t, families)

	for _, family := range families {
		assert.NotEqual(t, dto.MetricType_UNTYPED, family.GetType(), family.GetName())
		assert.Equal(t, *metricType(family.GetName()).ToDTO(), family.GetType(), family.GetName())
	}
}
