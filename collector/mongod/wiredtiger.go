package collector_mongod

import(
	"github.com/prometheus/client_golang/prometheus"
)

var (
	wtBlockManagerBlocksTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_blockmanager",
		Name:		"blocks_total",
		Help:		"TBD",
	}, []string{"type"})
	wtBlockManagerBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_blockmanager",
		Name:		"bytes_total",
		Help:		"TBD",
	}, []string{"type"})
)

var (
	wtCachePagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"pages_total",
		Help:		"TBD",
	}, []string{"type"})
	wtCacheBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"bytes_total",
		Help:		"TBD",
	}, []string{"type"})
	wtCacheEvictedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"evicted_total",
		Help:		"TBD",
	}, []string{"type"})
	wtCacheCurPages = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"current_pages",
		Help:		"TBD",
	})
	wtCacheBytesCached = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"bytes_cached",
		Help:		"TBD",
	})
	wtCacheBytesMax = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"bytes_max",
		Help:		"TBD",
	})
	wtCachePercentOverhead = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_cache",
		Name:		"percent_overhead",
		Help:		"TBD",
	})
)

var(
	wtTransactionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_transactions",
		Name:		"total",
		Help:		"TBD",
	}, []string{"type"})
	wtTransactionsTotalCheckpointMs = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_transactions",
		Name:		"total_chkp_ms",
		Help:		"TBD",
	})
	wtTransactionsCheckpointsRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_transactions",
		Name:		"chkp_running",
		Help:		"TBD",
	})
)

var(
	wtLogBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
                Namespace:      Namespace,
                Subsystem:      "wiredtiger_log",
                Name:           "bytes_total",
                Help:           "The total number of bytes written to the WiredTiger log",
        }, []string{"type"})
	wtLogOperationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
                Namespace:      Namespace,
                Subsystem:      "wiredtiger_log",
                Name:           "operations_total",
                Help:           "The total number of WiredTiger log operations",
        }, []string{"type"})
)

var(
	wtConcurrentTransactionsOut = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:      Namespace,
		Subsystem:      "wiredtiger_concur_transactions",
		Name:	   	"out",
		Help:	   	"TBD",
	}, []string{"type"})
	wtConcurrentTransactionsAvailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_concur_transactions",
		Name:		"available",
		Help:		"TBD",
	}, []string{"type"})
	wtConcurrentTransactionsTotalTickets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_concur_transactions",
		Name:		"tickets_total",
		Help:		"TBD",
	}, []string{"type"})
)

var(
	wtAsyncWorkQueueLength = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_async",
		Name:		"work_queue_length",
		Help:		"TBD",
	})
	wtAsyncMaxWorkQueueLength = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:	Namespace,
		Subsystem:	"wiredtiger_async",
		Name:		"max_work_queue_length",
		Help:		"TBD",
	})
)

// async stats
type WTAsyncStats struct {
	NumAllocStateRaces		float64	`bson:"number of allocation state races"`
	NumOpSlotsViewedForAlloc	float64	`bson:"number of operation slots viewed for allocation"`
	WorkQueueLength			float64	`bson:"current work queue length"`
	NumFlushCalls			float64	`bson:"number of flush calls"`
	NumAllocFailed			float64	`bson:"number of times operation allocation failed"`
	MaxWorkQueueLength		float64	`bson:"maximum work queue length"`
	NumWorkerNoWork			float64	`bson:"number of times worker found no work"`
	TotalAlloc			float64	`bson:"total allocations"`
	TotalCompact			float64	`bson:"total compact calls"`
	TotalInsert			float64	`bson:"total insert calls"`
	TotalRemove			float64	`bson:"total remove calls"`
	TotalSearch			float64	`bson:"total search calls"`
	TotalUpdate			float64	`bson:"total update calls"`
}

func (stats *WTAsyncStats) Export(ch chan<- prometheus.Metric) {
	wtAsyncWorkQueueLength.Set(stats.WorkQueueLength)
	wtAsyncMaxWorkQueueLength.Set(stats.MaxWorkQueueLength)
}

func (stats *WTAsyncStats) Describe(ch chan<- *prometheus.Desc) {
	wtAsyncWorkQueueLength.Describe(ch)
	wtAsyncMaxWorkQueueLength.Describe(ch)
}

// blockmanager stats
type WTBlockManagerStats struct {
	MappedBytesRead			float64	`bson:"mapped bytes read"`
	BytesRead			float64 `bson:"bytes read"`
	BytesWritten			float64 `bson:"bytes written"`
	MappedBlocksRead		float64 `bson:"mapped blocks read"`
	BlocksPreLoaded			float64 `bson:"blocks pre-loaded"`
	BlocksRead			float64 `bson:"blocks read"`
	BlocksWritten			float64 `bson:"blocks written"`
}

func (stats *WTBlockManagerStats) Export(ch chan<- prometheus.Metric) {
	wtBlockManagerBlocksTotal.WithLabelValues("read").Set(stats.BlocksRead)
	wtBlockManagerBlocksTotal.WithLabelValues("read_mapped").Set(stats.MappedBlocksRead)
	wtBlockManagerBlocksTotal.WithLabelValues("written").Set(stats.BlocksWritten)
	wtBlockManagerBytesTotal.WithLabelValues("read").Set(stats.BytesRead)
	wtBlockManagerBytesTotal.WithLabelValues("read_mapped").Set(stats.MappedBytesRead)
	wtBlockManagerBytesTotal.WithLabelValues("written").Set(stats.BytesWritten)
}

func (stats *WTBlockManagerStats) Describe(ch chan<- *prometheus.Desc) {
	wtBlockManagerBlocksTotal.Describe(ch)
	wtBlockManagerBytesTotal.Describe(ch)
}

// cache stats
type WTCacheStats struct {
	BytesCached			float64 `bson:"bytes currently in the cache"`
	BytesMaximum			float64	`bson:"maximum bytes configured"`
	BytesReadInto			float64 `bson:"bytes read into cache"`
	BytesWrittenFrom		float64 `bson:"bytes written from cache"`
	EvictedUnmodified		float64 `bson:"unmodified pages evicted"`
	EvictedModified			float64 `bson:"modified pages evicted"`
	PercentOverhead			float64 `bson:"percentage overhead"`
	PagesTotal			float64 `bson:"pages currently held in the cache"`
	PagesReadInto			float64 `bson:"pages read into cache"`
	PagesWrittenFrom		float64 `bson:"pages written from cache"`
}

func (stats *WTCacheStats) Export(ch chan<- prometheus.Metric) {
	wtCachePagesTotal.WithLabelValues("read").Set(stats.PagesReadInto)
	wtCachePagesTotal.WithLabelValues("written").Set(stats.PagesWrittenFrom)
	wtCacheBytesTotal.WithLabelValues("read").Set(stats.BytesReadInto)
	wtCacheBytesTotal.WithLabelValues("written").Set(stats.BytesWrittenFrom)
	wtCacheEvictedTotal.WithLabelValues("modified").Set(stats.EvictedModified)
	wtCacheEvictedTotal.WithLabelValues("unmodified").Set(stats.EvictedUnmodified)
	wtCacheCurPages.Set(stats.PagesTotal)
	wtCacheBytesCached.Set(stats.BytesCached)
	wtCacheBytesMax.Set(stats.BytesMaximum)
	wtCachePercentOverhead.Set(stats.PercentOverhead)
}

func (stats *WTCacheStats) Describe(ch chan<- *prometheus.Desc) {
	wtCachePagesTotal.Describe(ch)
	wtCacheEvictedTotal.Describe(ch)
	wtCacheCurPages.Describe(ch)
	wtCacheBytesCached.Describe(ch)
	wtCacheBytesMax.Describe(ch)
	wtCachePercentOverhead.Describe(ch)
}

// connection stats
type WTConnectionStats struct {
	OpenFiles			float64 `bson:"files currently open"`
	// the slash in "I/Os" breaks bson's flag parser (fixme)
	//TotalReadIOs			float64 `bson:"total read I/Os"`
	//TotalWriteIOs			float64 `bson:"total write I/Os"`
}

// cursor stats
type WTCursorStats struct {
	CreateCalls			float64 `bson:"cursor create calls"`
	InsertCalls			float64 `bson:"cursor insert calls"`
	NextCalls			float64 `bson:"cursor next calls"`
	PrevCalls			float64 `bson:"cursor prev calls"`
	RemoveCalls			float64 `bson:"cursor remove calls"`
	ResetCalls			float64 `bson:"cursor reset calls"`
	SearchCalls			float64 `bson:"cursor search calls"`
	SearchNearCalls			float64 `bson:"cursor search near calls"`
	UpdateCalls			float64 `bson:"cursor update calls"`
}

// log stats
type WTLogStats struct {
	TotalBufferSize			float64 `bson:"total log buffer size"`
	BytesPayloadData		float64 `bson:"log bytes of payload data"`
	BytesWritten			float64 `bson:"log bytes written"`
	RecordsUncompressed		float64 `bson:"log records not compressed"`
	RecordsCompressed		float64 `bson:"log records compressed"`
	LogFlushes			float64 `bson:"log flush operations"`
	MaxLogSize			float64 `bson:"maximum log file size"`
	LogReads			float64 `bson:"log read operations"`
	LogScansDouble			float64 `bson:"log scan records requiring two reads"`
	LogScans			float64 `bson:"log scan operations"`
	LogSyncs			float64 `bson:"log sync operations"`
	LogSyncDirs			float64 `bson:"log sync_dir operations"`
	LogWrites			float64 `bson:"log write operations"`
}

func (stats *WTLogStats) Export(ch chan<- prometheus.Metric) {
        wtLogBytesTotal.WithLabelValues("payload").Set(stats.BytesPayloadData)
        wtLogBytesTotal.WithLabelValues("written").Set(stats.BytesWritten)
        wtLogOperationsTotal.WithLabelValues("read").Set(stats.LogReads)
        wtLogOperationsTotal.WithLabelValues("write").Set(stats.LogWrites)
        wtLogOperationsTotal.WithLabelValues("scan").Set(stats.LogScans)
        wtLogOperationsTotal.WithLabelValues("scan_double").Set(stats.LogScansDouble)
        wtLogOperationsTotal.WithLabelValues("sync").Set(stats.LogSyncs)
        wtLogOperationsTotal.WithLabelValues("sync_dir").Set(stats.LogSyncDirs)
        wtLogOperationsTotal.WithLabelValues("flush").Set(stats.LogFlushes)
}

func (stats *WTLogStats) Describe(ch chan<- *prometheus.Desc) {
	wtLogBytesTotal.Describe(ch)
	wtLogOperationsTotal.Describe(ch)
}

// session stats
type WTSessionStats struct {
	Cursors				float64	`bson:"open cursor count"`
	Sessions			float64	`bson:"open session count"`
}

// transaction stats
type WTTransactionStats struct {
	Begins				float64 `bson:"transaction begins"`
	Checkpoints			float64 `bson:"transaction checkpoints"`
	CheckpointsRunning		float64 `bson:"transaction checkpoint currently running"`
	CheckpointMaxMs			float64 `bson:"transaction checkpoint max time (msecs)"`
	CheckpointMinMs			float64 `bson:"transaction checkpoint min time (msecs)"`
	CheckpointLastMs		float64 `bson:"transaction checkpoint most recent time (msecs)"`
	CheckpointTotalMs		float64 `bson:"transaction checkpoint total time (msecs)"`
	Committed			float64 `bson:"transactions committed"`
	CacheOverflowFailure		float64 `bson:"transaction failures due to cache overflow"`
	RolledBack			float64 `bson:"transactions rolled back"`
}

func (stats *WTTransactionStats) Export(ch chan<- prometheus.Metric) {
	wtTransactionsTotal.WithLabelValues("begins").Set(stats.Begins)
	wtTransactionsTotal.WithLabelValues("checkpoints").Set(stats.Checkpoints)
	wtTransactionsTotal.WithLabelValues("committed").Set(stats.Committed)
	wtTransactionsTotal.WithLabelValues("rolledback").Set(stats.RolledBack)
	wtTransactionsTotalCheckpointMs.Set(stats.CheckpointTotalMs)
	wtTransactionsCheckpointsRunning.Set(stats.CheckpointsRunning)
}

func (stats *WTTransactionStats) Describe(ch chan<- *prometheus.Desc) {
	wtTransactionsTotal.Describe(ch)
	wtTransactionsTotalCheckpointMs.Describe(ch)
	wtTransactionsCheckpointsRunning.Describe(ch)
}

// concurrenttransaction stats
type WTConcurrentTransactionsTypeStats struct {
	Out				float64 `bson:"out"`
	Available			float64 `bson:"available"`
	TotalTickets			float64 `bson:"totalTickets"`
}

type WTConcurrentTransactionsStats struct {
	Write	*WTConcurrentTransactionsTypeStats	`bson:"read"`
	Read	*WTConcurrentTransactionsTypeStats	`bson:"write"`
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
	Async			*WTAsyncStats			`bson:"async"`
	BlockManager		*WTBlockManagerStats		`bson:"block-manager"`
	Cache			*WTCacheStats			`bson:"cache"`
	Connection		*WTConnectionStats		`bson:"connection"`
	Cursor			*WTCursorStats			`bson:"cursor"`
	Log			*WTLogStats			`bson:"log"`
	Session			*WTSessionStats			`bson:"session"`
	Transaction		*WTTransactionStats		`bson:"transaction"`
	ConcurrentTransactions	*WTConcurrentTransactionsStats	`bson:"concurrentTransactions"`
}

func (stats *WiredTigerStats) Describe(ch chan<- *prometheus.Desc) {
	if stats.Async != nil {
		stats.Async.Describe(ch)
	}
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
	if stats.ConcurrentTransactions != nil {
		stats.ConcurrentTransactions.Describe(ch)
	}

	wtBlockManagerBlocksTotal.Describe(ch)
	wtBlockManagerBytesTotal.Describe(ch)

	wtCachePagesTotal.Describe(ch)
	wtCacheBytesTotal.Describe(ch)
	wtCacheEvictedTotal.Describe(ch)
	wtCacheCurPages.Describe(ch)
	wtCacheBytesCached.Describe(ch)
	wtCacheBytesMax.Describe(ch)
	wtCachePercentOverhead.Describe(ch)

	wtTransactionsTotal.Describe(ch)
	wtTransactionsTotalCheckpointMs.Describe(ch)
	wtTransactionsCheckpointsRunning.Describe(ch)

	wtConcurrentTransactionsOut.Describe(ch)
	wtConcurrentTransactionsAvailable.Describe(ch)
	wtConcurrentTransactionsTotalTickets.Describe(ch)

	wtAsyncWorkQueueLength.Describe(ch)
	wtAsyncMaxWorkQueueLength.Describe(ch)
}

func (stats *WiredTigerStats) Export(ch chan<- prometheus.Metric) {
	if stats.Async != nil {
		stats.Async.Export(ch)
	}
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
	if stats.ConcurrentTransactions != nil {
		stats.ConcurrentTransactions.Export(ch)
	}

	wtBlockManagerBlocksTotal.Collect(ch)
	wtBlockManagerBytesTotal.Collect(ch)

	wtCachePagesTotal.Collect(ch)
	wtCacheBytesTotal.Collect(ch)
	wtCacheEvictedTotal.Collect(ch)
	wtCacheCurPages.Collect(ch)
	wtCacheBytesCached.Collect(ch)
	wtCacheBytesMax.Collect(ch)
	wtCachePercentOverhead.Collect(ch)

	wtTransactionsTotal.Collect(ch)
	wtTransactionsTotalCheckpointMs.Collect(ch)
	wtTransactionsCheckpointsRunning.Collect(ch)

	wtLogBytesTotal.Collect(ch)
	wtLogOperationsTotal.Collect(ch)

	wtConcurrentTransactionsOut.Collect(ch)
	wtConcurrentTransactionsAvailable.Collect(ch)
	wtConcurrentTransactionsTotalTickets.Collect(ch)

	wtAsyncWorkQueueLength.Collect(ch)
	wtAsyncMaxWorkQueueLength.Collect(ch)
}
