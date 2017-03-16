package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsCursorTimedOutTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_cursor", "timed_out_total"),
		"timedOut provides the total number of cursors that have timed out since the server process started. If this number is large or growing at a regular rate, this may indicate an application error",
	  nil, nil)
)
var (
	metricsCursorOpen = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "metrics_cursor_open",
		Help:      "The open is an embedded document that contains data regarding open cursors",
	}, []string{"state"})
)
var (
	metricsDocumentTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "metrics_document_total"),
		"The document holds a document of that reflect document access and modification patterns and data use. Compare these values to the data in the opcounters document, which track total number of operations",
	  []string{"state"}, nil)
)
var (
	metricsGetLastErrorWtimeNumTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "metrics_get_last_error_wtime",
		Name:      "num_total",
		Help:      "num reports the total number of getLastError operations with a specified write concern (i.e. w) that wait for one or more members of a replica set to acknowledge the write operation (i.e. a w value greater than 1.)",
	})
	metricsGetLastErrorWtimeTotalMilliseconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_get_last_error_wtime", "total_milliseconds"),
		"total_millis reports the total amount of time in milliseconds that the mongod has spent performing getLastError operations with write concern (i.e. w) that wait for one or more members of a replica set to acknowledge the write operation (i.e. a w value greater than 1.)",
	  nil, nil)
)
var (
	metricsGetLastErrorWtimeoutsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_get_last_error", "wtimeouts_total"),
		"wtimeouts reports the number of times that write concern operations have timed out as a result of the wtimeout threshold to getLastError.",
	  nil, nil)
)
var (
	metricsOperationTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "metrics_operation_total"),
		"operation is a sub-document that holds counters for several types of update and query operations that MongoDB handles using special operation types",
	  []string{"type"}, nil)
)
var (
	metricsQueryExecutorTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "metrics_query_executor_total"),
		"queryExecutor is a document that reports data from the query execution system",
	  []string{"state"}, nil)
)
var (
	metricsRecordMovesTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_record", "moves_total"),
	  "moves reports the total number of times documents move within the on-disk representation of the MongoDB data set. Documents move as a result of operations that increase the size of the document beyond their allocated record size",
		nil, nil)
)
var (
	metricsReplApplyBatchesNumTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_apply_batches", "num_total"),
	  "num reports the total number of batches applied across all databases",
		nil, nil)
	metricsReplApplyBatchesTotalMilliseconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_apply_batches", "total_milliseconds"),
	  "total_millis reports the total amount of time the mongod has spent applying operations from the oplog",
		nil, nil)
)
var (
	metricsReplApplyOpsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_apply", "ops_total"),
	  "ops reports the total number of oplog operations applied",
		nil, nil)
)
var (
	metricsReplBufferCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "metrics_repl_buffer",
		Name:      "count",
		Help:      "count reports the current number of operations in the oplog buffer",
	})
	metricsReplBufferMaxSizeBytes = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_buffer", "max_size_bytes"),
	  "maxSizeBytes reports the maximum size of the buffer. This value is a constant setting in the mongod, and is not configurable",
		nil, nil)
	metricsReplBufferSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "metrics_repl_buffer",
		Name:      "size_bytes",
		Help:      "sizeBytes reports the current size of the contents of the oplog buffer",
	})
)
var (
	metricsReplNetworkGetmoresNumTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_network_getmores", "num_total"),
	  "num reports the total number of getmore operations, which are operations that request an additional set of operations from the replication sync source.",
		nil, nil)
	metricsReplNetworkGetmoresTotalMilliseconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_network_getmores", "total_milliseconds"),
	  "total_millis reports the total amount of time required to collect data from getmore operations",
		nil, nil)
)
var (
	metricsReplNetworkBytesTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_network", "bytes_total"),
	  "bytes reports the total amount of data read from the replication sync source",
		nil, nil)
	metricsReplNetworkOpsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_network", "ops_total"),
	  "ops reports the total number of operations read from the replication source.",
		nil, nil)
	metricsReplNetworkReadersCreatedTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_network", "readers_created_total"),
	  "readersCreated reports the total number of oplog query processes created. MongoDB will create a new oplog query any time an error occurs in the connection, including a timeout, or a network operation. Furthermore, readersCreated will increment every time MongoDB selects a new source fore replication.",
		nil, nil)
)
var (
	metricsReplPreloadDocsNumTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_preload_docs", "num_total"),
	  "num reports the total number of documents loaded during the pre-fetch stage of replication",
		nil, nil)
	metricsReplPreloadDocsTotalMilliseconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_preload_docs", "total_milliseconds"),
	  "total_millis reports the total amount of time spent loading documents as part of the pre-fetch stage of replication",
		nil, nil)
)
var (
	metricsReplPreloadIndexesNumTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_preload_indexes", "num_total"),
	  "num reports the total number of index entries loaded by members before updating documents as part of the pre-fetch stage of replication",
		nil, nil)
	metricsReplPreloadIndexesTotalMilliseconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metrics_repl_preload_indexes", "total_milliseconds"),
	  "total_millis reports the total amount of time spent loading index entries as part of the pre-fetch stage of replication",
		nil, nil)
)
var (
	metricsStorageFreelistSearchTotal = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "metrics_storage_freelist_search_total"),
		"metrics about searching records in the database.",
	  []string{"type"}, nil)
)

// DocumentStats are the stats associated to a document.
type DocumentStats struct {
	Deleted  float64 `bson:"deleted"`
	Inserted float64 `bson:"inserted"`
	Returned float64 `bson:"returned"`
	Updated  float64 `bson:"updated"`
}

// Export exposes the document stats to be consumed by the prometheus server.
func (documentStats *DocumentStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsDocumentTotal, prometheus.CounterValue, documentStats.Deleted, "deleted")
	ch <- prometheus.MustNewConstMetric(metricsDocumentTotal, prometheus.CounterValue, documentStats.Inserted, "inserted")
	ch <- prometheus.MustNewConstMetric(metricsDocumentTotal, prometheus.CounterValue, documentStats.Returned, "returned")
	ch <- prometheus.MustNewConstMetric(metricsDocumentTotal, prometheus.CounterValue, documentStats.Updated, "updated")
}

// BenchmarkStats is bechmark info about an operation.
type BenchmarkStats struct {
	Num         float64 `bson:"num"`
	TotalMillis float64 `bson:"totalMillis"`
}

// GetLastErrorStats are the last error stats.
type GetLastErrorStats struct {
	Wtimeouts float64         `bson:"wtimeouts"`
	Wtime     *BenchmarkStats `bson:"wtime"`
}

// Export exposes the get last error stats.
func (getLastErrorStats *GetLastErrorStats) Export(ch chan<- prometheus.Metric) {
	metricsGetLastErrorWtimeNumTotal.Set(getLastErrorStats.Wtime.Num)
	ch <- prometheus.MustNewConstMetric(metricsGetLastErrorWtimeTotalMilliseconds, prometheus.CounterValue, getLastErrorStats.Wtime.TotalMillis)

	ch <- prometheus.MustNewConstMetric(metricsGetLastErrorWtimeoutsTotal, prometheus.CounterValue, getLastErrorStats.Wtimeouts)
}

// OperationStats are the stats for some kind of operations.
type OperationStats struct {
	Fastmod      float64 `bson:"fastmod"`
	Idhack       float64 `bson:"idhack"`
	ScanAndOrder float64 `bson:"scanAndOrder"`
}

// Export exports the operation stats.
func (operationStats *OperationStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsOperationTotal, prometheus.CounterValue, operationStats.Fastmod, "fastmod")
	ch <- prometheus.MustNewConstMetric(metricsOperationTotal, prometheus.CounterValue, operationStats.Idhack, "idhack")
	ch <- prometheus.MustNewConstMetric(metricsOperationTotal, prometheus.CounterValue, operationStats.ScanAndOrder, "scan_and_order")
}

// QueryExecutorStats are the stats associated with a query execution.
type QueryExecutorStats struct {
	Scanned        float64 `bson:"scanned"`
	ScannedObjects float64 `bson:"scannedObjects"`
}

// Export exports the query executor stats.
func (queryExecutorStats *QueryExecutorStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsQueryExecutorTotal, prometheus.CounterValue, queryExecutorStats.Scanned, "scanned")
	ch <- prometheus.MustNewConstMetric(metricsQueryExecutorTotal, prometheus.CounterValue, queryExecutorStats.ScannedObjects, "scanned_objects")
}

// RecordStats are stats associated with a record.
type RecordStats struct {
	Moves float64 `bson:"moves"`
}

// Export exposes the record stats.
func (recordStats *RecordStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsRecordMovesTotal, prometheus.CounterValue, recordStats.Moves)
}

// ApplyStats are the stats associated with the apply operation.
type ApplyStats struct {
	Batches *BenchmarkStats `bson:"batches"`
	Ops     float64         `bson:"ops"`
}

// Export exports the apply stats
func (applyStats *ApplyStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsReplApplyOpsTotal, prometheus.CounterValue, applyStats.Ops)

	ch <- prometheus.MustNewConstMetric(metricsReplApplyBatchesNumTotal, prometheus.CounterValue, applyStats.Batches.Num)
	ch <- prometheus.MustNewConstMetric(metricsReplApplyBatchesTotalMilliseconds, prometheus.CounterValue, applyStats.Batches.TotalMillis)
}

// BufferStats are the stats associated with the buffer
type BufferStats struct {
	Count        float64 `bson:"count"`
	MaxSizeBytes float64 `bson:"maxSizeBytes"`
	SizeBytes    float64 `bson:"sizeBytes"`
}

// Export exports the buffer stats.
func (bufferStats *BufferStats) Export(ch chan<- prometheus.Metric) {
	metricsReplBufferCount.Set(bufferStats.Count)
	ch <- prometheus.MustNewConstMetric(metricsReplBufferMaxSizeBytes, prometheus.CounterValue, bufferStats.MaxSizeBytes)
	metricsReplBufferSizeBytes.Set(bufferStats.SizeBytes)
}

// MetricsNetworkStats are the network stats.
type MetricsNetworkStats struct {
	Bytes          float64         `bson:"bytes"`
	Ops            float64         `bson:"ops"`
	GetMores       *BenchmarkStats `bson:"getmores"`
	ReadersCreated float64         `bson:"readersCreated"`
}

// Export exposes the network stats.
func (metricsNetworkStats *MetricsNetworkStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsReplNetworkBytesTotal, prometheus.CounterValue, metricsNetworkStats.Bytes)
	ch <- prometheus.MustNewConstMetric(metricsReplNetworkOpsTotal, prometheus.CounterValue, metricsNetworkStats.Ops)
	ch <- prometheus.MustNewConstMetric(metricsReplNetworkReadersCreatedTotal, prometheus.CounterValue, metricsNetworkStats.ReadersCreated)

	ch <- prometheus.MustNewConstMetric(metricsReplNetworkGetmoresNumTotal, prometheus.CounterValue, metricsNetworkStats.GetMores.Num)
	ch <- prometheus.MustNewConstMetric(metricsReplNetworkGetmoresTotalMilliseconds, prometheus.CounterValue, metricsNetworkStats.GetMores.TotalMillis)
}

// ReplStats are the stats associated with the replication process.
type ReplStats struct {
	Apply        *ApplyStats          `bson:"apply"`
	Buffer       *BufferStats         `bson:"buffer"`
	Network      *MetricsNetworkStats `bson:"network"`
	PreloadStats *PreloadStats        `bson:"preload"`
}

// Export exposes the replication stats.
func (replStats *ReplStats) Export(ch chan<- prometheus.Metric) {
	replStats.Apply.Export(ch)
	replStats.Buffer.Export(ch)
	replStats.Network.Export(ch)
	replStats.PreloadStats.Export(ch)
}

// PreloadStats are the stats associated with preload operation.
type PreloadStats struct {
	Docs    *BenchmarkStats `bson:"docs"`
	Indexes *BenchmarkStats `bson:"indexes"`
}

// Export exposes the preload stats.
func (preloadStats *PreloadStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsReplPreloadDocsNumTotal, prometheus.CounterValue, preloadStats.Docs.Num)
	ch <- prometheus.MustNewConstMetric(metricsReplPreloadDocsTotalMilliseconds, prometheus.CounterValue, preloadStats.Docs.TotalMillis)

	ch <- prometheus.MustNewConstMetric(metricsReplPreloadIndexesNumTotal, prometheus.CounterValue, preloadStats.Indexes.Num)
	ch <- prometheus.MustNewConstMetric(metricsReplPreloadIndexesTotalMilliseconds, prometheus.CounterValue, preloadStats.Indexes.TotalMillis)
}

// StorageStats are the stats associated with the storage.
type StorageStats struct {
	BucketExhausted float64 `bson:"freelist.search.bucketExhausted"`
	Requests        float64 `bson:"freelist.search.requests"`
	Scanned         float64 `bson:"freelist.search.scanned"`
}

// Export exports the storage stats.
func (storageStats *StorageStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsStorageFreelistSearchTotal, prometheus.CounterValue, storageStats.BucketExhausted, "bucket_exhausted")
	ch <- prometheus.MustNewConstMetric(metricsStorageFreelistSearchTotal, prometheus.CounterValue, storageStats.Requests, "requests")
	ch <- prometheus.MustNewConstMetric(metricsStorageFreelistSearchTotal, prometheus.CounterValue, storageStats.Scanned, "scanned")
}

// CursorStatsOpen are the stats for open cursors
type CursorStatsOpen struct {
	NoTimeout	float64	`bson:"noTimeout"`
	Pinned		float64 `bson:"pinned"`
	Total		float64 `bson:"total"`
}

// CursorStats are the stats for cursors
type CursorStats struct {
	TimedOut	float64			`bson:"timedOut"`
	Open		*CursorStatsOpen	`bson:"open"`
}

// Export exports the cursor stats.
func (cursorStats *CursorStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricsCursorTimedOutTotal, prometheus.CounterValue, cursorStats.TimedOut)
	metricsCursorOpen.WithLabelValues("noTimeout").Set(cursorStats.Open.NoTimeout)
	metricsCursorOpen.WithLabelValues("pinned").Set(cursorStats.Open.Pinned)
	metricsCursorOpen.WithLabelValues("total").Set(cursorStats.Open.Total)
}

// MetricsStats are all stats associated with metrics of the system
type MetricsStats struct {
	Document      *DocumentStats      `bson:"document"`
	GetLastError  *GetLastErrorStats  `bson:"getLastError"`
	Operation     *OperationStats     `bson:"operation"`
	QueryExecutor *QueryExecutorStats `bson:"queryExecutor"`
	Record        *RecordStats        `bson:"record"`
	Repl          *ReplStats          `bson:"repl"`
	Storage       *StorageStats       `bson:"storage"`
	Cursor        *CursorStats        `bson:"cursor"`
}

// Export exports the metrics stats.
func (metricsStats *MetricsStats) Export(ch chan<- prometheus.Metric) {
	if metricsStats.Document != nil {
		metricsStats.Document.Export(ch)
	}
	if metricsStats.GetLastError != nil {
		metricsStats.GetLastError.Export(ch)
	}
	if metricsStats.Operation != nil {
		metricsStats.Operation.Export(ch)
	}
	if metricsStats.QueryExecutor != nil {
		metricsStats.QueryExecutor.Export(ch)
	}
	if metricsStats.Record != nil {
		metricsStats.Record.Export(ch)
	}
	if metricsStats.Repl != nil {
		metricsStats.Repl.Export(ch)
	}
	if metricsStats.Storage != nil {
		metricsStats.Storage.Export(ch)
	}
	if metricsStats.Cursor != nil {
		metricsStats.Cursor.Export(ch)
	}

	metricsCursorOpen.Collect(ch)
	metricsGetLastErrorWtimeNumTotal.Collect(ch)
	metricsReplBufferCount.Collect(ch)
	metricsReplBufferSizeBytes.Collect(ch)
}

// Describe describes the metrics for prometheus
func (metricsStats *MetricsStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- metricsCursorTimedOutTotal
	metricsCursorOpen.Describe(ch)
	ch <- metricsDocumentTotal
	metricsGetLastErrorWtimeNumTotal.Describe(ch)
	ch <- metricsGetLastErrorWtimeTotalMilliseconds
	ch <- metricsGetLastErrorWtimeoutsTotal
	ch <- metricsOperationTotal
	ch <- metricsQueryExecutorTotal
	ch <- metricsRecordMovesTotal
	ch <- metricsReplApplyBatchesNumTotal
	ch <- metricsReplApplyBatchesTotalMilliseconds
	ch <- metricsReplApplyOpsTotal
	metricsReplBufferCount.Describe(ch)
	ch <- metricsReplBufferMaxSizeBytes
	metricsReplBufferSizeBytes.Describe(ch)
	ch <- metricsReplNetworkGetmoresNumTotal
	ch <- metricsReplNetworkGetmoresTotalMilliseconds
	ch <- metricsReplNetworkBytesTotal
	ch <- metricsReplNetworkOpsTotal
	ch <- metricsReplNetworkReadersCreatedTotal
	ch <- metricsReplPreloadDocsNumTotal
	ch <- metricsReplPreloadDocsTotalMilliseconds
	ch <- metricsReplPreloadIndexesNumTotal
	ch <- metricsReplPreloadIndexesTotalMilliseconds
	ch <- metricsStorageFreelistSearchTotal
}
