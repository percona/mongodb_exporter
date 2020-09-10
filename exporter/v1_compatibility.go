package exporter

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidMetricPath  = fmt.Errorf("invalid metric path")
	ErrInvalidMetricValue = fmt.Errorf("invalid metric value")
)

/*
  This is used to convert a new metric like: mongodb_ss_asserts{assert_type=*} (1)
  to the old-compatible metric:  mongodb_mongod_asserts_total{type="regular|warning|msg|user|rollovers"}.
  In this particular case, conversion would be:
  conversion {
   newName: "mongodb_ss_asserts",
   oldName: "mongodb_mongod_asserts_total",
   labels : map[string]string{ "assert_type": "type"},
  }.

  Some other metric renaming are more complex. (2)
  In some cases, there is a total renaming, with new labels and the only part we can use to identify a metric
  is its prefix. Example:
  Metrics like mongodb_ss_metrics_operation _fastmod or
  mongodb_ss_metrics_operation_idhack or
  mongodb_ss_metrics_operation_scanAndOrder
  should use the trim the "prefix" mongodb_ss_metrics_operation from the metric name, and that remaining suffic
  is the label value for a new label "suffixLabel".
  It means that the metric (current) mongodb_ss_metrics_operation_idhack will become into the old equivalent one
  mongodb_mongod_metrics_operation_total {"state": "idhack"} as defined in the conversion slice:
   {
     oldName:     "mongodb_mongod_metrics_operation_total", //{state="fastmod|idhack|scan_and_order"}
     prefix:      "mongodb_ss_metrics_operation",           // _[fastmod|idhack|scanAndOrder]
     suffixLabel: "state",
   },

   suffixMapping field:
   --------------------
   Also, some metrics suffixes for the second renaming case need a mapping between the old and new values.
   For example, the metric mongodb_ss_wt_cache_bytes_currently_in_the_cache has mongodb_ss_wt_cache_bytes
   as the prefix so the suffix is bytes_currently_in_the_cache should be converted to a mertic named
   mongodb_mongod_wiredtiger_cache_bytes and the suffix bytes_currently_in_the_cache is being mapped to
   "total".

   Third renaming form: see (3) below.
*/

// For simple metric renaming, only some fields should be updated like the metric name, the help and some
// labels that have 1 to 1 mapping (1).
func newToOldMetric(rm *rawMetric, c conversion) *rawMetric {
	oldMetric := &rawMetric{
		fqName: c.oldName,
		help:   rm.help,
		val:    rm.val,
		vt:     rm.vt,
		ln:     make([]string, 0, len(rm.ln)),
		lv:     make([]string, 0, len(rm.lv)),
	}

	// Label values remain the same
	// copy(oldMetric.lv, rm.lv)

	for _, val := range rm.lv {
		if newLabelVal, ok := c.labelValueConversions[val]; ok {
			oldMetric.lv = append(oldMetric.lv, newLabelVal)
			continue
		}
		oldMetric.lv = append(oldMetric.lv, val)
	}

	// Some label names should be converted from the new (current) name to the
	// mongodb_exporter v1 compatible name
	for _, newLabelName := range rm.ln {
		// if it should be converted, append the old-compatible name
		if oldLabel, ok := c.labelConversions[newLabelName]; ok {
			oldMetric.ln = append(oldMetric.ln, oldLabel)
			continue
		}
		// otherwise, keep the same label name
		oldMetric.ln = append(oldMetric.ln, newLabelName)
	}

	return oldMetric
}

// The second renaming case is not a direct rename. In this case, the new metric name has a common
// prefix and the rest of the metric name is used as the value for a label in tne old metric style. (2)
// In this renaming case, the metric "mongodb_ss_wt_cache_bytes_bytes_currently_in_the_cache
// should be converted to mongodb_mongod_wiredtiger_cache_bytes with label "type": "total".
// For this conversion, we have the suffixMapping field that holds the mapping for all suffixes.
// Example definition:
//	 	oldName:     "mongodb_mongod_wiredtiger_cache_bytes",
//	 	prefix:      "mongodb_ss_wt_cache_bytes",
//	 	suffixLabel: "type",
//	 	suffixMapping: map[string]string{
//	 		"bytes_currently_in_the_cache":                           "total",
//	 		"tracked_dirty_bytes_in_the_cache":                       "dirty",
//	 		"tracked_bytes_belonging_to_internal_pages_in_the_cache": " internal_pages",
//	 		"tracked_bytes_belonging_to_leaf_pages_in_the_cache":     "internal_pages",
//	 	},
//	 },
func createOldMetricFromNew(rm *rawMetric, c conversion) *rawMetric {
	suffix := strings.TrimPrefix(rm.fqName, c.prefix)
	suffix = strings.TrimPrefix(suffix, "_")

	if newSuffix, ok := c.suffixMapping[suffix]; ok {
		suffix = newSuffix
	}

	oldMetric := &rawMetric{
		fqName: c.oldName,
		help:   c.oldName,
		val:    rm.val,
		vt:     rm.vt,
		ln:     []string{c.suffixLabel},
		lv:     []string{suffix},
	}

	return oldMetric
}

func cacheEvictedTotalMetric(m bson.M) (prometheus.Metric, error) {
	s, err := sumMetrics(m, [][]string{
		{"serverStatus", "wiredTiger", "cache", "modified pages evicted"},
		{"serverStatus", "wiredTiger", "cache", "unmodified pages evicted"},
	})
	if err != nil {
		return nil, err
	}

	d := prometheus.NewDesc("mongodb_mongod_wiredtiger_cache_evicted_total", "wiredtiger cache evicted total", nil, nil)
	metric, err := prometheus.NewConstMetric(d, prometheus.GaugeValue, s)
	if err != nil {
		return nil, err
	}

	return metric, nil
}

func sumMetrics(m bson.M, paths [][]string) (float64, error) {
	var total float64

	for _, path := range paths {
		v := walkTo(m, path)
		if v == nil {
			return 0, errors.Wrapf(ErrInvalidMetricPath, "%v", path)
		}

		f, err := asFloat64(v)
		if err != nil {
			return 0, errors.Wrapf(ErrInvalidMetricValue, "%v", v)
		}

		total += *f
	}

	return total, nil
}

// Converts new metric to the old metric style and append it to the response slice.
func appendCompatibleMetric(res []prometheus.Metric, rm *rawMetric) []prometheus.Metric {
	compatibleMetric := metricRenameAndLabel(rm, conversions())
	if compatibleMetric == nil {
		return res
	}

	metric, err := rawToPrometheusMetric(compatibleMetric)
	if err != nil {
		invalidMetric := prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
		res = append(res, invalidMetric)
		return res
	}

	res = append(res, metric)
	return res
}

func conversions() []conversion {
	return []conversion{
		{
			newName:          "mongodb_ss_asserts",
			oldName:          "mongodb_asserts_total",
			labelConversions: map[string]string{"assert_type": "type"},
		},
		{
			oldName:          "mongodb_asserts_total",
			newName:          "mongodb_ss_asserts",
			labelConversions: map[string]string{"assert_type": "type"},
		},
		{
			oldName:          "mongodb_connections",
			newName:          "mongodb_ss_connections",
			labelConversions: map[string]string{"conn_type": "state"},
		},
		{
			oldName: "mongodb_connections_metrics_created_total",
			newName: "mongodb_ss_connections_totalCreated",
		},
		{
			oldName: "mongodb_mongod_extra_info_page_faults_total",
			newName: "mongodb_ss_extra_info_page_faults",
		},
		{
			oldName:     "mongodb_mongod_global_lock_client",   // {type="readers|writers"}
			prefix:      "mongodb_ss_globalLock_activeClients", // _[readers|writers]
			suffixLabel: "type",
		},
		{
			oldName:          "mongodb_mongod_global_lock_current_queue",
			newName:          "mongodb_ss_globalLock_currentQueue",
			labelConversions: map[string]string{"count_type": "type"},
			labelValueConversions: map[string]string{
				"readers": "reader",
				"writers": "writer",
			},
		},
		{
			oldName: "mongodb_instance_local_time",
			newName: "mongodb_start",
		},

		{
			oldName: "mongodb_instance_uptime_seconds",
			newName: "mongodb_ss_uptime",
		},
		{
			oldName: "mongodb_mongod_locks_time_locked_local_microseconds_total", //{database=*,lock_type="read|write"}
			newName: "mongodb_ss_locks_Local_acquireCount_[rw]",
		},
		{
			oldName: "mongodb_memory", //{"resident|virtual|mapped|mapped_with_journal"}
			newName: "mongodb_ss_mem_[resident|virtual]",
		},
		{
			oldName:          "mongodb_mongod_metrics_cursor_open", //{state="noTimeout|pinned|total"}
			newName:          "mongodb_ss_metrics_cursor_open",     //{csr_type="noTimeout|pinned|total""}
			labelConversions: map[string]string{"csr_type": "state"},
		},
		{
			oldName: "mongodb_mongod_metrics_cursor_timed_out_total",
			newName: "mongodb_ss_metrics_cursor_timedOut",
		},
		{
			oldName:          "mongodb_mongod_metrics_document_total",
			newName:          "mongodb_ss_metric_document",
			labelConversions: map[string]string{"doc_op_type": "type"},
		},
		{
			oldName: "mongodb_mongod_metrics_get_last_error_wtime_num_total",
			newName: "mongodb_ss_metrics_getLastError_wtime_num",
		},
		{
			oldName: "mongodb_mongod_metrics_get_last_error_wtimeouts_total",
			newName: "mongodb_ss_metrics_getLastError_wtimeouts",
		},
		{
			oldName:     "mongodb_mongod_metrics_operation_total", //{state="fastmod|idhack|scan_and_order"}
			prefix:      "mongodb_ss_metrics_operation",           // _[fastmod|idhack|scanAndOrder] (I'm pretty sure fastmod is deprecated; idhack might be deprecated too.
			suffixLabel: "state",
		},
		{
			oldName:     "mongodb_mongod_metrics_query_executor_total", //{state="scanned|scannedObjects"}
			prefix:      "mongodb_ss_metrics_query",                    // _[scanned|scannedObjects]
			suffixLabel: "state",
		},
		{
			oldName: "mongodb_mongod_metrics_record_moves_total",
			newName: "mongodb_ss_metrics_record_moves",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_apply_batches_num_total",
			newName: "mongodb_ss_metrics_repl_apply_batches_num",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_apply_batches_total_milliseconds",
			newName: "mongodb_ss_metrics_repl_apply_batches_totalMillis",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_apply_ops_total",
			newName: "mongodb_ss_metrics_repl_apply_ops",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_buffer_count",
			newName: "mongodb_ss_metrics_repl_buffer_count",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_buffer_max_size_bytes",
			newName: "mongodb_ss_metrics_repl_buffer_maxSizeBytes",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_buffer_size_bytes",
			newName: "mongodb_ss_metrics_repl_buffer_sizeBytes",
		},
		{
			oldName:     "mongodb_mongod_metrics_repl_executor_queue", //{type=*}
			prefix:      "mongodb_ss_metrics_repl_executor_queues",    //_[networkInProgress|sleepers]
			suffixLabel: "type",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_executor_unsignaled_events",
			newName: "mongodb_ss_metrics_repl_executor_unsignaledEvents",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_network_bytes_total",
			newName: "mongodb_ss_metrics_repl_network_bytes",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_network_getmores_num_total",
			newName: "mongodb_ss_metrics_repl_network_getmores_num",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_network_getmores_total_milliseconds",
			newName: "mongodb_ss_metrics_repl_network_getmores_totalMillis",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_network_ops_total",
			newName: "mongodb_ss_metrics_repl_network_ops",
		},
		{
			oldName: "mongodb_mongod_metrics_repl_network_readers_created_total",
			newName: "mongodb_ss_metrics_repl_network_readersCreated",
		},
		{
			oldName: "mongodb_mongod_metrics_ttl_deleted_documents_total",
			newName: "mongodb_ss_metrics_ttl_deletedDocuments",
		},
		{
			oldName: "mongodb_mongod_metrics_ttl_passes_total",
			newName: "mongodb_ss_metrics_ttl_passes",
		},
		{
			oldName:     "mongodb_network_bytes_total", // {state="in_bytes|out_bytes"}
			prefix:      "mongodb_ss_network",          //_[bytesIn|bytesOut]
			suffixLabel: "state",
		},
		{
			oldName: "mongodb_network_metrics_num_requests_total",
			newName: "mongodb_ss_network_numRequests",
		},
		{
			oldName:          "mongodb_mongod_op_counters_repl_total",
			newName:          "mongodb_ss_opcountersRepl",
			labelConversions: map[string]string{"legacy_op_type": "type"},
		},
		{
			oldName:          "mongodb_mongod_op_counters_total", // {type=*}
			newName:          "mongodb_ss_opcounters",            //{legacy_op_type=*}
			labelConversions: map[string]string{"legacy_op_type": "type"},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_blockmanager_blocks_total", //{type="read|read_mapped|pre_loaded|written"}
			prefix:      "mongodb_ss_wt_block_manager",                         //_[blocks_read|mapped_blocks_read|blocks_written|blocks_pre_loaded]
			suffixLabel: "type",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_cache_max_bytes",
			newName: "mongodb_ss_wt_cache_maximum_bytes_configured",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_cache_overhead_percent",
			newName: "mongodb_ss_wt_cache_percentage_overhead",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_concurrent_transactions_available_tickets",
			newName: "mongodb_ss_wt_concurrentTransactions_available",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_concurrent_transactions_out_tickets",
			newName: "mongodb_ss_wt_concurrentTransactions_out",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_concurrent_transactions_total_tickets",
			newName: "mongodb_ss_wt_concurrentTransactions_totalTickets",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_log_records_scanned_total",
			newName: "mongodb_ss_wt_log_records_processed_by_log_scan",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_session_open_cursors_total",
			newName: "mongodb_ss_wt_session_open_cursor_count",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_session_open_sessions_total",
			newName: "mongodb_ss_wt_session_open_session_count",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_transactions_checkpoint_milliseconds_total",
			newName: "mongodb_ss_wt_txn_transaction_checkpoint_total_time_msecs",
		},
		{
			oldName: "mongodb_mongod_wiredtiger_transactions_running_checkpoints",
			newName: "mongodb_ss_wt_txn_transaction_checkpoint_currently_running",
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_transactions_total", // {type="begins|checkpoints|committed|rolledback"}
			prefix:      "mongodb_ss_wt_txn_transactions",               //_[begins|checkpoints|committed|rolled_back]
			suffixLabel: "type",
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_blockmanager_bytes_total", //{type=read|read_mapped|written"}
			prefix:      "mongodb_ss_wt_block_manager",                        //_[bytes_read|mapped_bytes_read|bytes_written]
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"bytes_read": "read", "mapped_bytes_read": "read_mapped",
				"bytes_written": "written",
			},
		},
		// the 2 metrics bellow have the same prefix.
		{
			oldName:     "mongodb_mongod_wiredtiger_cache_bytes", //{type="total|dirty|internal_pages|leaf_pages"}
			prefix:      "mongodb_ss_wt_cache_bytes",             //_[bytes_currently_in_the_cache|tracked_dirty_bytes_in_the_cache|tracked_bytes_belonging_to_internal_pages_in_the_cache|tracked_bytes_belonging_to_leaf_pages_in_the_cache]
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"bytes_currently_in_the_cache":                           "total",
				"tracked_dirty_bytes_in_the_cache":                       "dirty",
				"tracked_bytes_belonging_to_internal_pages_in_the_cache": " internal_pages",
				"tracked_bytes_belonging_to_leaf_pages_in_the_cache":     "internal_pages",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_cache_bytes_total",
			prefix:      "mongodb_ss_wt_cache",
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"bytes_read_into_cache":    "read",
				"bytes_written_from_cache": "written",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_cache_bytes_total", //{type="read", "written"}
			prefix:      "mongodb_ss_wt_cache",                         //_[bytes read into cache|bytes written from cache]
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"bytes_read_into_cache":    "read",
				"bytes_written_from_cache": "written",
			},
		},
		// {
		// 	oldName:     "mongodb_mongod_wiredtiger_cache_evicted_total", //{type="modified|unmodified"}
		// 	prefix:      "mongodb_ss_wt_cache",                           //_[modified pages evicted|unmodified_pages_evicted]
		// 	suffixLabel: "type",
		// 	suffixMapping: map[string]string{
		// 		"modified_pages_evicted":   "modified",
		// 		"unmodified_pages_evicted": "unmodified",
		// 	},
		// },
		{
			oldName:     "mongodb_mongod_wiredtiger_cache_pages", //{type="total|dirty"}
			prefix:      "mongodb_ss_wt_cache",                   //_[pages_currently_held_in_the_cache|tracked_dirty_pages_in_the_cache]
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"pages_currently_held_in_the_cache": "total",
				"tracked_dirty_pages_in_the_cache":  "dirty",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_cache_pages_total", //{type="read|written"}
			prefix:      "mongodb_ss_wt_cache",                         //_[pages_read_into_cache|pages_written_from_cache]
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"pages_read_into_cache":    "read",
				"pages_written_from_cache": "written",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_log_records_total", //{type="compressed|uncompressed"}
			prefix:      "mongodb_ss_wt_log",                           //_[log records compressed|log_records_not_compressed]
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"log_records_compressed":     "compressed",
				"log_records_not_compressed": "uncompressed",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_log_bytes_total", //{type="payload|unwritten"}
			prefix:      "mongodb_ss_wt_log",                         //_[log_bytes_of_payload_data|log_bytes_written
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"log_bytes_of_payload_data": "payload",
				"log_bytes_written":         "unwritten",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_log_operations_total", //{type="
			prefix:      "mongodb_ss_wt_log",
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"log_read_operations":                  "read",
				"log_write_operations":                 "write",
				"log_scan_operations":                  "scan",
				"log_scan_records_requiring_two_reads": "scan_double",
				"log_sync_operations":                  "sync",
				"log_sync_dir_operations":              "sync_dir",
				"log_flush_operations":                 "flush",
			},
		},
		{
			oldName:     "mongodb_mongod_wiredtiger_transactions_checkpoint_milliseconds", //{type="min|max"}
			prefix:      "mongodb_ss_wt_txn_transaction_checkpoint",                       //_[min|max]_time_msecs
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"min_time_msecs": "min",
				"max_time_msecs": "max",
			},
		},
		// New metrics PMM-6610
		// mongodb_mongod_global_lock_current_queue {type="reader"}  mongodb_mongod_global_lock_current_queue {type="readers"}
		// mongodb_mongod_global_lock_current_queue {type="writer"}  mongodb_mongod_global_lock_current_queue {type="writers"}
		{
			oldName:          "mongodb_mongod_global_lock_current_queue",
			prefix:           "mongodb_mongod_global_lock_current_queue",
			labelConversions: map[string]string{"op_type": "type"},
		},
		// mongodb_mongod_op_latencies_ops_total {type="command"}  	 mongodb_ss_opLatencies_ops{op_type="commands"}
		// mongodb_mongod_op_latencies_ops_total{type="write"}	     mongodb_ss_opLatencies_ops{op_type="writes"}
		{
			oldName:          "mongodb_mongod_op_latencies_ops_total",
			prefix:           "mongodb_ss_opLatencies_ops",
			labelConversions: map[string]string{"op_type": "type"},
			labelValueConversions: map[string]string{
				"commands": "command",
			},
		},
		// mongodb_mongod_metrics_document_total {state="deleted"} 	 mongodb_ss_metrics_document {doc_op_type="deleted"}
		{
			oldName:          "mongodb_mongod_metrics_document_total",
			newName:          "mongodb_ss_metrics_document",
			labelConversions: map[string]string{"doc_op_type": "state"},
		},
		{
			// mongodb_mongod_metrics_query_executor_total {state="scanned"}	        mongodb_ss_metrics_queryExecutor_scanned
			// mongodb_mongod_metrics_query_executor_total {state="scanned_objects"} 	mongodb_ss_metrics_queryExecutor_scannedObjects
			oldName:     "mongodb_mongod_metrics_query_executor_total",
			prefix:      "mongodb_ss_metrics_queryExecutor",
			suffixLabel: "state",
			suffixMapping: map[string]string{
				"scanned":        "scanned",
				"scannedObjects": "scanned_objects",
			},
		},
		{
			oldName:          "mongodb_mongod_op_latencies_latency_total",
			newName:          "mongodb_ss_opLatencies_latency",
			labelConversions: map[string]string{"op_type": "type"},
			labelValueConversions: map[string]string{
				"commands":     "command",
				"reads":        "read",
				"transactions": "transaction",
				"writes":       "write",
			},
		},
		// PMM-6610 Naylia comments
		{
			oldName:     "mongodb_memory",
			prefix:      "mongodb_ss_mem",
			suffixLabel: "type",
			suffixMapping: map[string]string{
				"resident": "resident",
				"virtual":  "virtual",
			},
		},
		{
			oldName: "mongodb_mongod_metrics_get_last_error_wtime_total_milliseconds",
			newName: "mongodb_ss_metrics_getLastError_wtime_totalMillis",
		},
		{
			oldName: "mongodb_ss_wt_cache_maximum_bytes_configured",
			newName: "mongodb_mongod_wiredtiger_cache_max_bytes",
		},
	}
}

// Third metric renaming case (3).
// Lock* metrics don't fit in (1) nor in (2) and since they are just a few, and we know they always exists
// as part of getDiagnosticData, we can just call locksMetrics with getDiagnosticData result as the input
// to get the v1 compatible metrics from the new structure.

type lockMetric struct {
	name   string
	path   []string
	labels map[string]string
}

func lockMetrics() []lockMetric {
	return []lockMetric{
		{
			name:   "mongodb_ss_locks_acquireCount",
			path:   []string{"serverStatus", "locks", "ParallelBatchWriterMode", "acquireCount", "r"},
			labels: map[string]string{"lock_mode": "r", "resource": "ParallelBatchWriterMode"},
		},
		{
			name:   "mongodb_ss_locks_acquireCount",
			path:   []string{"serverStatus", "locks", "ParallelBatchWriterMode", "acquireCount", "w"},
			labels: map[string]string{"lock_mode": "w", "resource": "ReplicationStateTransition"},
		},
		{
			name:   "mongodb_ss_locks_acquireCount",
			path:   []string{"serverStatus", "locks", "ReplicationStateTransition", "acquireCount", "w"},
			labels: map[string]string{"resource": "ReplicationStateTransition", "lock_mode": "w"},
		},
		{
			name:   "mongodb_ss_locks_acquireWaitCount",
			path:   []string{"serverStatus", "locks", "ReplicationStateTransition", "acquireCount", "W"},
			labels: map[string]string{"lock_mode": "W", "resource": "ReplicationStateTransition"},
		},
		{
			name:   "mongodb_ss_locks_timeAcquiringMicros",
			path:   []string{"serverStatus", "locks", "ReplicationStateTransition", "timeAcquiringMicros", "w"},
			labels: map[string]string{"lock_mode": "w", "resource": "ReplicationStateTransition"},
		},
		{
			name:   "mongodb_ss_locks_acquireCount",
			path:   []string{"serverStatus", "locks", "Global", "acquireCount", "r"},
			labels: map[string]string{"lock_mode": "r", "resource": "Global"},
		},
		{
			name:   "mongodb_ss_locks_acquireCount",
			path:   []string{"serverStatus", "locks", "Global", "acquireCount", "w"},
			labels: map[string]string{"lock_mode": "w", "resource": "Global"},
		},
		{
			name:   "mongodb_ss_locks_acquireCount",
			path:   []string{"serverStatus", "locks", "Global", "acquireCount", "W"},
			labels: map[string]string{"lock_mode": "W", "resource": "Global"},
		},
	}
}

// locksMetrics returns the list of lock metrics as a prometheus.Metric slice
// This function reads the human readable list from lockMetrics() and creates a slice of metrics
// ready to be exposed, taking the value for each metric from th provided bson.M structure from
// getDiagnosticData.
func locksMetrics(m bson.M) []prometheus.Metric {
	res := make([]prometheus.Metric, 0, len(lockMetrics()))

	for _, lm := range lockMetrics() {
		mm, err := makeLockMetric(m, lm)
		if mm == nil {
			continue
		}
		if err != nil {
			logrus.Errorf("cannot convert lock metric %s to old style: %s", mm.Desc(), err)
			continue
		}
		res = append(res, mm)
	}

	return res
}

func makeLockMetric(m bson.M, lm lockMetric) (prometheus.Metric, error) {
	val := walkTo(m, lm.path)
	if val == nil {
		return nil, nil
	}

	f, err := asFloat64(val)
	if err != nil {
		return prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err), err
	}

	if f == nil {
		return nil, nil
	}

	ln := make([]string, 0, len(lm.labels))
	lv := make([]string, 0, len(lm.labels))
	for labelName, labelValue := range lm.labels {
		ln = append(ln, labelName)
		lv = append(lv, labelValue)
	}

	d := prometheus.NewDesc(lm.name, lm.name, ln, nil)

	return prometheus.NewConstMetric(d, prometheus.UntypedValue, *f, lv...)
}

// PMM dashboards looks for this metric so, in compatibility mode, we must expose it.
// FIXME Add it in both modes, move away from that file: https://jira.percona.com/browse/PMM-6585
func mongodbUpMetric() prometheus.Metric {
	d := prometheus.NewDesc("mongodb_up", "Whether MongoDB is up.", nil, nil)
	up, err := prometheus.NewConstMetric(d, prometheus.GaugeValue, float64(1))
	if err != nil {
		panic(err)
	}
	return up
}

func walkTo(m primitive.M, path []string) interface{} {
	val, ok := m[path[0]]
	if !ok {
		return nil
	}

	if len(path) > 1 {
		switch v := val.(type) {
		case primitive.M:
			val = walkTo(v, path[1:])
		case map[string]interface{}:
			val = walkTo(v, path[1:])
		default:
			return nil
		}
	}

	return val
}
