// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	wtBlockManagerBlocksTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_blockmanager", "blocks_total"),
		"The total number of blocks read by the WiredTiger BlockManager",
		[]string{"type"},
		nil,
	)
	wtBlockManagerBytesTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_blockmanager", "bytes_total"),
		"The total number of bytes read by the WiredTiger BlockManager",
		[]string{"type"},
		nil,
	)
)

var (
	wtCachePages = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_cache",
		Name:      "pages",
		Help:      "The current number of pages in the WiredTiger Cache",
	}, []string{"type"})
	wtCachePagesTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_cache", "pages_total"),
		"The total number of pages read into/from the WiredTiger Cache",
		[]string{"type"},
		nil,
	)
	wtCacheBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_cache",
		Name:      "bytes",
		Help:      "The current size of data in the WiredTiger Cache in bytes",
	}, []string{"type"})
	wtCacheMaxBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_cache",
		Name:      "max_bytes",
		Help:      "The maximum size of data in the WiredTiger Cache in bytes",
	})
	wtCacheBytesTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_cache", "bytes_total"),
		"The total number of bytes read into/from the WiredTiger Cache",
		[]string{"type"},
		nil,
	)
	wtCacheEvictedTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_cache", "evicted_total"),
		"The total number of pages evicted from the WiredTiger Cache",
		[]string{"type"},
		nil,
	)
	wtCachePercentOverhead = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_cache",
		Name:      "overhead_percent",
		Help:      "The percentage overhead of the WiredTiger Cache",
	})
)

var (
	wtTransactionsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_transactions", "total"),
		"The total number of transactions WiredTiger has handled",
		[]string{"type"},
		nil,
	)
	wtTransactionsTotalCheckpointMsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_transactions", "checkpoint_milliseconds_total"),
		"The total time in milliseconds transactions have checkpointed in WiredTiger",
		nil,
		nil,
	)
	wtTransactionsCheckpointMs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_transactions",
		Name:      "checkpoint_milliseconds",
		Help:      "The time in milliseconds transactions have checkpointed in WiredTiger",
	}, []string{"type"})
	wtTransactionsCheckpointsRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_transactions",
		Name:      "running_checkpoints",
		Help:      "The number of currently running checkpoints in WiredTiger",
	})
)

var (
	wtLogRecordsScannedTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_log", "records_scanned_total"),
		"The total number of records scanned by log scan in the WiredTiger log",
		nil,
		nil,
	)
	wtLogRecordsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_log", "records_total"),
		"The total number of compressed/uncompressed records written to the WiredTiger log",
		[]string{"type"},
		nil,
	)
	wtLogBytesTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_log", "bytes_total"),
		"The total number of bytes written to the WiredTiger log",
		[]string{"type"},
		nil,
	)
	wtLogOperationsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "wiredtiger_log", "operations_total"),
		"The total number of WiredTiger log operations",
		[]string{"type"},
		nil,
	)
)

var (
	wtOpenCursors = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_session",
		Name:      "open_cursors_total",
		Help:      "The total number of cursors opened in WiredTiger",
	})
	wtOpenSessions = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_session",
		Name:      "open_sessions_total",
		Help:      "The total number of sessions opened in WiredTiger",
	})
)

var (
	wtConcurrentTransactionsOut = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_concurrent_transactions",
		Name:      "out_tickets",
		Help:      "The number of tickets that are currently in use (out) in WiredTiger",
	}, []string{"type"})
	wtConcurrentTransactionsAvailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_concurrent_transactions",
		Name:      "available_tickets",
		Help:      "The number of tickets that are available in WiredTiger",
	}, []string{"type"})
	wtConcurrentTransactionsTotalTickets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "wiredtiger_concurrent_transactions",
		Name:      "total_tickets",
		Help:      "The total number of tickets that are available in WiredTiger",
	}, []string{"type"})
)

// blockmanager stats
type WTBlockManagerStats struct {
	MappedBytesRead  float64 `bson:"mapped bytes read"`
	BytesRead        float64 `bson:"bytes read"`
	BytesWritten     float64 `bson:"bytes written"`
	MappedBlocksRead float64 `bson:"mapped blocks read"`
	BlocksPreLoaded  float64 `bson:"blocks pre-loaded"`
	BlocksRead       float64 `bson:"blocks read"`
	BlocksWritten    float64 `bson:"blocks written"`
}

func (stats *WTBlockManagerStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(wtBlockManagerBlocksTotalDesc, prometheus.CounterValue, stats.BlocksRead, "read")
	ch <- prometheus.MustNewConstMetric(wtBlockManagerBlocksTotalDesc, prometheus.CounterValue, stats.MappedBlocksRead, "read_mapped")
	ch <- prometheus.MustNewConstMetric(wtBlockManagerBlocksTotalDesc, prometheus.CounterValue, stats.BlocksPreLoaded, "pre_loaded")
	ch <- prometheus.MustNewConstMetric(wtBlockManagerBlocksTotalDesc, prometheus.CounterValue, stats.BlocksWritten, "written")

	ch <- prometheus.MustNewConstMetric(wtBlockManagerBytesTotalDesc, prometheus.CounterValue, stats.BytesRead, "read")
	ch <- prometheus.MustNewConstMetric(wtBlockManagerBytesTotalDesc, prometheus.CounterValue, stats.MappedBytesRead, "read_mapped")
	ch <- prometheus.MustNewConstMetric(wtBlockManagerBytesTotalDesc, prometheus.CounterValue, stats.BytesWritten, "written")
}

func (stats *WTBlockManagerStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- wtBlockManagerBlocksTotalDesc
	ch <- wtBlockManagerBytesTotalDesc
}

// cache stats
type WTCacheStats struct {
	BytesTotal         float64 `bson:"bytes currently in the cache"`
	BytesDirty         float64 `bson:"tracked dirty bytes in the cache"`
	BytesInternalPages float64 `bson:"tracked bytes belonging to internal pages in the cache"`
	BytesLeafPages     float64 `bson:"tracked bytes belonging to leaf pages in the cache"`
	MaxBytes           float64 `bson:"maximum bytes configured"`
	BytesReadInto      float64 `bson:"bytes read into cache"`
	BytesWrittenFrom   float64 `bson:"bytes written from cache"`
	EvictedUnmodified  float64 `bson:"unmodified pages evicted"`
	EvictedModified    float64 `bson:"modified pages evicted"`
	PercentOverhead    float64 `bson:"percentage overhead"`
	PagesTotal         float64 `bson:"pages currently held in the cache"`
	PagesReadInto      float64 `bson:"pages read into cache"`
	PagesWrittenFrom   float64 `bson:"pages written from cache"`
	PagesDirty         float64 `bson:"tracked dirty pages in the cache"`
}

func (stats *WTCacheStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(wtCachePagesTotalDesc, prometheus.CounterValue, stats.PagesReadInto, "read")
	ch <- prometheus.MustNewConstMetric(wtCachePagesTotalDesc, prometheus.CounterValue, stats.PagesWrittenFrom, "written")
	ch <- prometheus.MustNewConstMetric(wtCacheBytesTotalDesc, prometheus.CounterValue, stats.BytesReadInto, "read")
	ch <- prometheus.MustNewConstMetric(wtCacheBytesTotalDesc, prometheus.CounterValue, stats.BytesWrittenFrom, "written")
	ch <- prometheus.MustNewConstMetric(wtCacheEvictedTotalDesc, prometheus.CounterValue, stats.EvictedModified, "modified")
	ch <- prometheus.MustNewConstMetric(wtCacheEvictedTotalDesc, prometheus.CounterValue, stats.EvictedUnmodified, "unmodified")

	wtCachePages.WithLabelValues("total").Set(stats.PagesTotal)
	wtCachePages.WithLabelValues("dirty").Set(stats.PagesDirty)
	wtCacheBytes.WithLabelValues("total").Set(stats.BytesTotal)
	wtCacheBytes.WithLabelValues("dirty").Set(stats.BytesDirty)
	wtCacheBytes.WithLabelValues("internal_pages").Set(stats.BytesInternalPages)
	wtCacheBytes.WithLabelValues("leaf_pages").Set(stats.BytesLeafPages)
	wtCacheMaxBytes.Set(stats.MaxBytes)
	wtCachePercentOverhead.Set(stats.PercentOverhead)
}

func (stats *WTCacheStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- wtCachePagesTotalDesc
	ch <- wtCacheEvictedTotalDesc
	wtCachePages.Describe(ch)
	wtCacheBytes.Describe(ch)
	wtCacheMaxBytes.Describe(ch)
	wtCachePercentOverhead.Describe(ch)
}

// log stats
type WTLogStats struct {
	TotalBufferSize         float64 `bson:"total log buffer size"`
	TotalSizeCompressed     float64 `bson:"total size of compressed records"`
	BytesPayloadData        float64 `bson:"log bytes of payload data"`
	BytesWritten            float64 `bson:"log bytes written"`
	RecordsUncompressed     float64 `bson:"log records not compressed"`
	RecordsCompressed       float64 `bson:"log records compressed"`
	RecordsProcessedLogScan float64 `bson:"records processed by log scan"`
	MaxLogSize              float64 `bson:"maximum log file size"`
	LogFlushes              float64 `bson:"log flush operations"`
	LogReads                float64 `bson:"log read operations"`
	LogScansDouble          float64 `bson:"log scan records requiring two reads"`
	LogScans                float64 `bson:"log scan operations"`
	LogSyncs                float64 `bson:"log sync operations"`
	LogSyncDirs             float64 `bson:"log sync_dir operations"`
	LogWrites               float64 `bson:"log write operations"`
}

func (stats *WTLogStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(wtLogRecordsTotalDesc, prometheus.CounterValue, stats.RecordsCompressed, "compressed")
	ch <- prometheus.MustNewConstMetric(wtLogRecordsTotalDesc, prometheus.CounterValue, stats.RecordsUncompressed, "uncompressed")

	ch <- prometheus.MustNewConstMetric(wtLogBytesTotalDesc, prometheus.CounterValue, stats.BytesPayloadData, "payload")
	ch <- prometheus.MustNewConstMetric(wtLogBytesTotalDesc, prometheus.CounterValue, stats.BytesWritten, "written")

	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogReads, "read")
	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogWrites, "write")
	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogScans, "scan")
	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogScansDouble, "scan_double")
	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogSyncs, "sync")
	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogSyncDirs, "sync_dir")
	ch <- prometheus.MustNewConstMetric(wtLogOperationsTotalDesc, prometheus.CounterValue, stats.LogFlushes, "flush")

	ch <- prometheus.MustNewConstMetric(wtLogRecordsScannedTotalDesc, prometheus.CounterValue, stats.RecordsProcessedLogScan)
}

func (stats *WTLogStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- wtLogRecordsTotalDesc
	ch <- wtLogBytesTotalDesc
	ch <- wtLogOperationsTotalDesc
	ch <- wtLogRecordsScannedTotalDesc
}

// session stats
type WTSessionStats struct {
	Cursors  float64 `bson:"open cursor count"`
	Sessions float64 `bson:"open session count"`
}

func (stats *WTSessionStats) Export(ch chan<- prometheus.Metric) {
	wtOpenCursors.Set(stats.Cursors)
	wtOpenSessions.Set(stats.Sessions)
}

func (stats *WTSessionStats) Describe(ch chan<- *prometheus.Desc) {
	wtOpenCursors.Describe(ch)
	wtOpenSessions.Describe(ch)
}

// transaction stats
type WTTransactionStats struct {
	Begins               float64 `bson:"transaction begins"`
	Checkpoints          float64 `bson:"transaction checkpoints"`
	CheckpointsRunning   float64 `bson:"transaction checkpoint currently running"`
	CheckpointMaxMs      float64 `bson:"transaction checkpoint max time (msecs)"`
	CheckpointMinMs      float64 `bson:"transaction checkpoint min time (msecs)"`
	CheckpointLastMs     float64 `bson:"transaction checkpoint most recent time (msecs)"`
	CheckpointTotalMs    float64 `bson:"transaction checkpoint total time (msecs)"`
	Committed            float64 `bson:"transactions committed"`
	CacheOverflowFailure float64 `bson:"transaction failures due to cache overflow"`
	RolledBack           float64 `bson:"transactions rolled back"`
}

func (stats *WTTransactionStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(wtTransactionsTotalDesc, prometheus.CounterValue, stats.Begins, "begins")
	ch <- prometheus.MustNewConstMetric(wtTransactionsTotalDesc, prometheus.CounterValue, stats.Checkpoints, "checkpoints")
	ch <- prometheus.MustNewConstMetric(wtTransactionsTotalDesc, prometheus.CounterValue, stats.Committed, "committed")
	ch <- prometheus.MustNewConstMetric(wtTransactionsTotalDesc, prometheus.CounterValue, stats.RolledBack, "rolledback")
	wtTransactionsCheckpointMs.WithLabelValues("min").Set(stats.CheckpointMinMs)
	wtTransactionsCheckpointMs.WithLabelValues("max").Set(stats.CheckpointMaxMs)
	ch <- prometheus.MustNewConstMetric(wtTransactionsTotalCheckpointMsDesc, prometheus.CounterValue, stats.CheckpointTotalMs)
	wtTransactionsCheckpointsRunning.Set(stats.CheckpointsRunning)
}

func (stats *WTTransactionStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- wtTransactionsTotalDesc
	ch <- wtTransactionsTotalCheckpointMsDesc
	wtTransactionsCheckpointMs.Describe(ch)
	wtTransactionsCheckpointsRunning.Describe(ch)
}

// concurrenttransaction stats
type WTConcurrentTransactionsTypeStats struct {
	Out          float64 `bson:"out"`
	Available    float64 `bson:"available"`
	TotalTickets float64 `bson:"totalTickets"`
}

type WTConcurrentTransactionsStats struct {
	Read  *WTConcurrentTransactionsTypeStats `bson:"read"`
	Write *WTConcurrentTransactionsTypeStats `bson:"write"`
}

func (stats *WTConcurrentTransactionsStats) Export(ch chan<- prometheus.Metric) {
	wtConcurrentTransactionsOut.WithLabelValues("read").Set(stats.Read.Out)
	wtConcurrentTransactionsOut.WithLabelValues("write").Set(stats.Write.Out)
	wtConcurrentTransactionsAvailable.WithLabelValues("read").Set(stats.Read.Available)
	wtConcurrentTransactionsAvailable.WithLabelValues("write").Set(stats.Write.Available)
	wtConcurrentTransactionsTotalTickets.WithLabelValues("read").Set(stats.Read.TotalTickets)
	wtConcurrentTransactionsTotalTickets.WithLabelValues("write").Set(stats.Write.TotalTickets)
}

func (stats *WTConcurrentTransactionsStats) Describe(ch chan<- *prometheus.Desc) {
	wtConcurrentTransactionsOut.Describe(ch)
	wtConcurrentTransactionsAvailable.Describe(ch)
	wtConcurrentTransactionsTotalTickets.Describe(ch)
}

// WiredTiger stats
type WiredTigerStats struct {
	BlockManager           *WTBlockManagerStats           `bson:"block-manager"`
	Cache                  *WTCacheStats                  `bson:"cache"`
	Log                    *WTLogStats                    `bson:"log"`
	Session                *WTSessionStats                `bson:"session"`
	Transaction            *WTTransactionStats            `bson:"transaction"`
	ConcurrentTransactions *WTConcurrentTransactionsStats `bson:"concurrentTransactions"`
}

func (stats *WiredTigerStats) Describe(ch chan<- *prometheus.Desc) {
	if stats.BlockManager != nil {
		stats.BlockManager.Describe(ch)
	}
	if stats.Cache != nil {
		stats.Cache.Describe(ch)
	}
	if stats.Transaction != nil {
		stats.Transaction.Describe(ch)
	}
	if stats.Log != nil {
		stats.Log.Describe(ch)
	}
	if stats.Session != nil {
		stats.Session.Describe(ch)
	}
	if stats.ConcurrentTransactions != nil {
		stats.ConcurrentTransactions.Describe(ch)
	}
}

func (stats *WiredTigerStats) Export(ch chan<- prometheus.Metric) {
	if stats.BlockManager != nil {
		stats.BlockManager.Export(ch)
	}
	if stats.Cache != nil {
		stats.Cache.Export(ch)
	}
	if stats.Transaction != nil {
		stats.Transaction.Export(ch)
	}
	if stats.Log != nil {
		stats.Log.Export(ch)
	}
	if stats.Session != nil {
		stats.Session.Export(ch)
	}
	if stats.ConcurrentTransactions != nil {
		stats.ConcurrentTransactions.Export(ch)
	}

	wtCachePages.Collect(ch)
	wtCacheBytes.Collect(ch)
	wtCacheMaxBytes.Collect(ch)
	wtCachePercentOverhead.Collect(ch)

	wtTransactionsCheckpointMs.Collect(ch)
	wtTransactionsCheckpointsRunning.Collect(ch)

	wtOpenCursors.Collect(ch)
	wtOpenSessions.Collect(ch)

	wtConcurrentTransactionsOut.Collect(ch)
	wtConcurrentTransactionsAvailable.Collect(ch)
	wtConcurrentTransactionsTotalTickets.Collect(ch)
}
