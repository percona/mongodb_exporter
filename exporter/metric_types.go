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

//go:generate go run wiredtiger_stats_gen.go

import (
	"regexp"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// typeRule assigns a type to the metric families whose name, without the exporter prefix,
// matches re.
type typeRule struct {
	re *regexp.Regexp
	vt prometheus.ValueType
}

func counter(re string) typeRule {
	return typeRule{re: regexp.MustCompile(re), vt: prometheus.CounterValue}
}

func gauge(re string) typeRule {
	return typeRule{re: regexp.MustCompile(re), vt: prometheus.GaugeValue}
}

// metricTypeRules lists the metric families MongoDB documents as cumulative, and the gauges
// that sit under them. The first matching rule wins. A family no rule matches is a gauge.
//
// Prometheus requires every series of a family to have the same type, so families that mix
// both kinds under a label, like mongodb_ss_connections{conn_type="current|totalCreated"},
// are gauges.
var metricTypeRules = []typeRule{
	// Histogram buckets, see processHistogramSlice
	counter(`_(histograms?|queueWaitTimeMicros)(_.+)?_count$`),

	// serverStatus
	counter(`^ss_(asserts|opcounters|opcountersRepl)$`),
	counter(`^ss_(opLatencies|opWorkingTime)(_|$)`),
	counter(`^ss_network_(bytesIn|bytesOut|physicalBytesIn|physicalBytesOut|numRequests|numSlowDNSOperations|numSlowSSLOperations|compression_.+|tcpFastOpen_accepted|connsRejectedDueToMaxPendingProxyProtocolHeader|listenerProcessingTime_durationMicros|ingressRequestRateLimiter_.+Admissions)$`),
	counter(`^ss_connections_establishmentRateLimit_(exempted|interruptedDueToClientDisconnect|rejected)$`),
	counter(`^ss_extra_info_(page_faults|page_reclaims|input_blocks|output_blocks|voluntary_context_switches|involuntary_context_switches|user_time_us|system_time_us|pressure_.+_totalMicros)$`),
	counter(`^ss_globalLock_totalTime$`),
	counter(`^ss_locks_(.+_)?(acquireCount|acquireWaitCount|timeAcquiringMicros|deadlockCount)(_.+)?$`),
	counter(`^ss_lockContentionMetrics_.+_(contentions|total|waitCycles)$`),
	gauge(`^ss_metrics_(cursor_open(_.+)?|repl_buffer_.+|repl_executor_.+|repl_stateTransition_.+|repl_waiters_(opTime|replication)|repl_network_oplogFetcherLagSeconds|query_planCacheTotalSizeEstimateBytes|query_planCache_(totalQueryShapes|totalSizeEstimateBytes)|query_updateDeleteManyDocumentsMaxCount|query_updateDeleteManyDurationMaxMs|queryStats_(queryStatsStoreSizeEstimateBytes|maxSizeBytes|numEntries|numPartitions)|network_averageTimeToCompleted.+|mongos_shardRegistry_blockedOpsGauge)$`),
	counter(`^ss_metrics_`),
	counter(`^ss_transactions_(total.+|retried.+|transactionsCollectionWriteCount|commitTypes(_.+)?|abortCause_.+)$`),
	counter(`^ss_shardingStatistics_(numHostsTargeted_.+|(.+_)?count[A-Z].*|(.+_)?total[A-Z].*)$`),
	counter(`^ss_hedgingMetrics_`),
	counter(`^ss_internalTransactions_(started|succeeded|retriedCommits|retriedTransactions)$`),
	counter(`^ss_electionMetrics_(.+_(called|successful)|num.+)$`),
	counter(`^ss_(readConcernCounters|readPreferenceCounters|opReadConcernCounters|opWriteConcernCounters)(_|$)`),
	counter(`^ss_security_authentication_.+`),
	counter(`^ss_scramCache_.+_(hits|misses)$`),
	counter(`^ss_mirroredReads_(seen|sent|processedAsSecondary|resolved|succeeded)$`),
	counter(`^ss_flowControl_(isLaggedCount|isLaggedTimeMicros|timeAcquiringMicros)$`),
	counter(`^ss_oplogTruncation_(totalTimeProcessingMicros|totalTimeTruncatingMicros|truncateCount)$`),
	counter(`^ss_oplogTruncationThread_interruptCount$`),
	counter(`^ss_indexBulkBuilder_(count|bytesSpilled|bytesSpilledUncompressed|filesClosedForExternalSort|filesOpenedForExternalSort|resumed|spilledRanges|numSorted|bytesSorted)$`),
	counter(`^ss_indexBuilds_(total|killedDueToInsufficientDiskSpace|failedDueToDataCorruption|phases_(scanCollection|drainSideWritesTable|drainSideWritesTablePreCommit|waitForCommitQuorum|drainSideWritesTableOnCommit|processConstraintsViolatonTableOnCommit|commit))$`),
	counter(`^ss_indexStats_features_.+_accesses$`),
	counter(`^ss_logicalSessionRecordCache_(sessionsCollectionJobCount|transactionReaperJobCount)$`),
	counter(`^ss_batchedDeletes_`),
	counter(`^ss_(twoPhaseCommitCoordinator|tenantMigrations|shardSplits)_total.+$`),
	counter(`^ss_transportSecurity_`),
	counter(`^ss_tcmalloc_tcmalloc_(pageheap_(commit|decommit|reserve|scavenge)_count|pageheap_total_(commit|decommit|reserve)_bytes|spinlock_total_delay_ns)$`),
	counter(`^ss_changeStreamPreImages_(docsInserted|purgingJob_(bytesDeleted|docsDeleted|scannedCollections|scannedInternalCollections|timeElapsedMillis|totalPass))$`),
	counter(`^ss_collectionCatalog_numScansDueToMissingMapping$`),
	counter(`^ss_fle_operationCounters_`),
	counter(`^ss_profiler_(totalWrites|totalAbandonedWrites)$`),
	counter(`^ss_queryAnalyzers_totalSampled(Reads|Writes)(Bytes|Count)$`),
	counter(`^ss_recoveryOplogApplier_(numBatches|numOpsApplied)$`),
	counter(`^ss_repl_userWriteBlockModeCounters_`),
	counter(`^ss_searchTaskExecutorMetrics_.+_connectionPool_(acquisitionWaitTimes_.+|totalCreated|totalRefreshed|totalWasNeverUsed)$`),
	counter(`^ss_searchTaskExecutorMetrics_.+_networkInterface_.+_(executed|scheduled|canceled|runTime_.+|waitTime_.+)$`),
	counter(`^ss_wt_perf$`),
	counter(`^ss_wt_concurrentTransactions_monitor_(timesDecreased|timesIncreased|totalAmountDecreased|totalAmountIncreased|resizeDurationMicros)$`),
	counter(`^ss_(wt_concurrentTransactions|queues)_.+_(addedToQueue|removedFromQueue|startedProcessing|finishedProcessing|totalTimeProcessingMicros|totalTimeQueuedMicros|canceled|newAdmissions|interruptedInQueue|.+Admissions)$`),
	counter(`^ss_wt_snapshot_window_settings_(total_number_of_SnapshotTooOld_errors|pinned_timestamp_requests)$`),
	counter(`^transportLayerStats_connsDiscardedDueToClientDisconnect$`),

	// getDiagnosticData systemMetrics
	counter(`^sys_cpu_(.+_ms|ctxt|processes)$`),
	counter(`^sys_disks_.+_(reads|writes|reads_merged|writes_merged|read_sectors|write_sectors|read_time_ms|write_time_ms|io_time_ms|io_queued_ms|discards|discards_merged|discard_sectors|discard_time_ms|flushes|flush_time_ms)$`),
	gauge(`^sys_netstat_(Tcp_(CurrEstab|RtoAlgorithm|RtoMin|RtoMax|MaxConn)|Ip_(Forwarding|DefaultTTL))$`),
	counter(`^sys_netstat_`),
	gauge(`^sys_vmstat_nr_`),
	counter(`^sys_vmstat_`),
	counter(`^sys_pressure_.+_totalMicros$`),

	// $collStats, $indexStats and top
	counter(`^collstats_latencyStats_`),
	counter(`^indexstats_accesses_ops$`),
	counter(`^top_.+_(count|time)$`),

	// Metric names of mongodb_exporter v0.11, exposed with --compatible-mode
	gauge(`^mongod_(db_.+|wiredtiger_session_open_.+)_total$`),
	counter(`^(mongod_.+|asserts|op_counters|network_bytes|network_metrics_num_requests|connections_metrics_created|extra_info_page_faults)_total(_milliseconds)?$`),
}

// wiredTigerFamilyMarkers precede the WiredTiger statistics in a metric family name:
// mongodb_ss_wt_cache_..., mongodb_oplog_stats_wt_cache_...,
// mongodb_collstats_storageStats_wiredTiger_cache_...,
// mongodb_collstats_storageStats_indexDetails_index_name_cache_... and so on.
var wiredTigerFamilyMarkers = []string{"_wt_historyStorageStats_", "_wt_", "_wiredTiger_", "_indexDetails_index_name_"}

var (
	wiredTigerTypesOnce sync.Once
	wiredTigerTypes     map[string]prometheus.ValueType
	metricTypeCache     sync.Map
)

// metricType returns the type of a metric family. The type depends on the family name only,
// so every series of a family gets the same type.
func metricType(fqName string) prometheus.ValueType {
	if vt, ok := metricTypeCache.Load(fqName); ok {
		return vt.(prometheus.ValueType) //nolint:forcetypeassert
	}

	vt := resolveMetricType(fqName)
	metricTypeCache.Store(fqName, vt)

	return vt
}

func resolveMetricType(fqName string) prometheus.ValueType {
	if vt, ok := wiredTigerStatType(fqName); ok {
		return vt
	}

	name := strings.TrimPrefix(fqName, exporterPrefix)
	for _, rule := range metricTypeRules {
		if rule.re.MatchString(name) {
			return rule.vt
		}
	}

	return prometheus.GaugeValue
}

// wiredTigerStatType returns the type WiredTiger gives the statistic a family reports, if it
// reports one.
func wiredTigerStatType(fqName string) (prometheus.ValueType, bool) {
	wiredTigerTypesOnce.Do(loadWiredTigerTypes)

	// serverStatus.wiredTiger.transaction is shortened to ss_wt_txn.
	if stat, ok := strings.CutPrefix(fqName, exporterPrefix+"ss_wt_txn_"); ok {
		vt, ok := wiredTigerTypes["transaction_"+stat]
		return vt, ok
	}

	for _, marker := range wiredTigerFamilyMarkers {
		if _, stat, ok := strings.Cut(fqName, marker); ok {
			if vt, ok := wiredTigerTypes[stat]; ok {
				return vt, true
			}
		}
	}

	return 0, false
}

func loadWiredTigerTypes() {
	wiredTigerTypes = make(map[string]prometheus.ValueType, len(wiredTigerCounters)+len(wiredTigerGauges))

	for _, stat := range wiredTigerCounters {
		wiredTigerTypes[wiredTigerStatKey(stat)] = prometheus.CounterValue
	}

	// Two statistics can share a name once special characters are replaced; a gauge wins.
	for _, stat := range wiredTigerGauges {
		wiredTigerTypes[wiredTigerStatKey(stat)] = prometheus.GaugeValue
	}
}

// wiredTigerStatKey turns "cache: bytes currently in the cache" into the
// cache_bytes_currently_in_the_cache suffix the statistic gets in metric names.
func wiredTigerStatKey(stat string) string {
	section, desc, _ := strings.Cut(stat, ": ")

	return normalizeMetricName(section + "." + desc)
}
