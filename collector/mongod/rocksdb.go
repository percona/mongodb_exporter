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
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// byte-size and unit constants:
	kilobyte float64 = 1024
	megabyte float64 = kilobyte * 1024
	gigabyte float64 = megabyte * 1024
	terabyte float64 = gigabyte * 1024
	petabyte float64 = terabyte * 1024
	thousand float64 = 1000
	million  float64 = thousand * 1000
	billion  float64 = million * 1000
	trillion float64 = billion * 1000

	rocksDbStalledSecsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "stalled_seconds_total"),
		"The total number of seconds RocksDB has spent stalled",
		nil,
		nil,
	)
	rocksDbStallsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "stalls_total"),
		"The total number of stalls in RocksDB",
		[]string{"type"},
		nil,
	)
	rocksDbCompactionBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "compaction_bytes_total"),
		"Total bytes processed during compaction between levels N and N+1 in RocksDB",
		[]string{"level", "type"},
		nil,
	)
	rocksDbCompactionSecondsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "compaction_seconds_total"),
		"The time spent doing compactions between levels N and N+1 in RocksDB",
		[]string{"level"},
		nil,
	)
	rocksDbCompactionsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "compactions_total"),
		"The total number of compactions between levels N and N+1 in RocksDB",
		[]string{"level", "type"},
		nil,
	)
	rocksDbBlockCacheHitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "block_cache_hits_total"),
		"The total number of hits to the RocksDB Block Cache",
		nil,
		nil,
	)
	rocksDbBlockCacheMissesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "block_cache_misses_total"),
		"The total number of misses to the RocksDB Block Cache",
		nil,
		nil,
	)
	rocksDbKeysDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "keys_total"),
		"The total number of RocksDB key operations",
		[]string{"type"},
		nil,
	)
	rocksDbSeeksDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "seeks_total"),
		"The total number of seeks performed by RocksDB",
		nil,
		nil,
	)
	rocksDbIterationsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "iterations_total"),
		"The total number of iterations performed by RocksDB",
		[]string{"type"},
		nil,
	)
	rocksDbBloomFilterUsefulDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "bloom_filter_useful_total"),
		"The total number of times the RocksDB Bloom Filter was useful",
		nil,
		nil,
	)
	rocksDbBytesWrittenDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "bytes_written_total"),
		"The total number of bytes written by RocksDB",
		[]string{"type"},
		nil,
	)
	rocksDbBytesReadDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "bytes_read_total"),
		"The total number of bytes read by RocksDB",
		[]string{"type"},
		nil,
	)
	rocksDbReadOpsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "rocksdb", "reads_total"),
		"The total number of read operations in RocksDB",
		[]string{"level"},
		nil,
	)
)

var (
	rocksDbNumImmutableMemTable = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "immutable_memtables",
		Help:      "The total number of immutable MemTables in RocksDB",
	})
	rocksDbMemTableFlushPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "pending_memtable_flushes",
		Help:      "The total number of MemTable flushes pending in RocksDB",
	})
	rocksDbCompactionPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "pending_compactions",
		Help:      "The total number of compactions pending in RocksDB",
	})
	rocksDbBackgroundErrors = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "background_errors",
		Help:      "The total number of background errors in RocksDB",
	})
	rocksDbMemTableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "memtable_bytes",
		Help:      "The current number of MemTable bytes in RocksDB",
	}, []string{"type"})
	rocksDbMemtableEntries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "memtable_entries",
		Help:      "The current number of Memtable entries in RocksDB",
	}, []string{"type"})
	rocksDbEstimateTableReadersMem = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "estimate_table_readers_memory_bytes",
		Help:      "The estimate RocksDB table-reader memory bytes",
	})
	rocksDbNumSnapshots = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "snapshots",
		Help:      "The current number of snapshots in RocksDB",
	})
	rocksDbOldestSnapshotTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "oldest_snapshot_timestamp",
		Help:      "The timestamp of the oldest snapshot in RocksDB",
	})
	rocksDbNumLiveVersions = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "live_versions",
		Help:      "The current number of live versions in RocksDB",
	})
	rocksDbTotalLiveRecoveryUnits = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "total_live_recovery_units",
		Help:      "The total number of live recovery units in RocksDB",
	})
	rocksDbBlockCacheUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "block_cache_bytes",
		Help:      "The current bytes used in the RocksDB Block Cache",
	})
	rocksDbTransactionEngineKeys = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "transaction_engine_keys",
		Help:      "The current number of transaction engine keys in RocksDB",
	})
	rocksDbTransactionEngineSnapshots = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "transaction_engine_snapshots",
		Help:      "The current number of transaction engine snapshots in RocksDB",
	})
	rocksDbWritesPerBatch = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "writes_per_batch",
		Help:      "The number of writes per batch in RocksDB",
	})
	rocksDbWritesPerSec = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "writes_per_second",
		Help:      "The number of writes per second in RocksDB",
	})
	rocksDbStallPercent = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "stall_percent",
		Help:      "The percentage of time RocksDB has been stalled",
	})
	rocksDbWALWritesPerSync = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "write_ahead_log_writes_per_sync",
		Help:      "The number of writes per Write-Ahead-Log sync in RocksDB",
	})
	rocksDbWALBytesPerSecs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "write_ahead_log_bytes_per_second",
		Help:      "The number of bytes written per second by the Write-Ahead-Log in RocksDB",
	})
	rocksDbLevelFiles = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "files",
		Help:      "The number of files in a RocksDB level",
	}, []string{"level"})
	rocksDbCompactionThreads = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "compaction_file_threads",
		Help:      "The number of threads currently doing compaction for levels in RocksDB",
	}, []string{"level"})
	rocksDbLevelScore = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "compaction_score",
		Help:      "The compaction score of RocksDB levels",
	}, []string{"level"})
	rocksDbLevelSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "size_bytes",
		Help:      "The total byte size of levels in RocksDB",
	}, []string{"level"})
	rocksDbCompactionBytesPerSec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "compaction_bytes_per_second",
		Help:      "The rate at which data is processed during compaction between levels N and N+1 in RocksDB",
	}, []string{"level", "type"})
	rocksDbCompactionWriteAmplification = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "compaction_write_amplification",
		Help:      "The write amplification factor from compaction between levels N and N+1 in RocksDB",
	}, []string{"level"})
	rocksDbCompactionAvgSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "compaction_average_seconds",
		Help:      "The average time per compaction between levels N and N+1 in RocksDB",
	}, []string{"level"})
	rocksDbReadLatencyMicros = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "rocksdb",
		Name:      "read_latency_microseconds",
		Help:      "The read latency in RocksDB in microseconds by level",
	}, []string{"level", "type"})
)

type RocksDbStatsCounters struct {
	NumKeysWritten         float64 `bson:"num-keys-written"`
	NumKeysRead            float64 `bson:"num-keys-read"`
	NumSeeks               float64 `bson:"num-seeks"`
	NumForwardIter         float64 `bson:"num-forward-iterations"`
	NumBackwardIter        float64 `bson:"num-backward-iterations"`
	BlockCacheMisses       float64 `bson:"block-cache-misses"`
	BlockCacheHits         float64 `bson:"block-cache-hits"`
	BloomFilterUseful      float64 `bson:"bloom-filter-useful"`
	BytesWritten           float64 `bson:"bytes-written"`
	BytesReadPointLookup   float64 `bson:"bytes-read-point-lookup"`
	BytesReadIteration     float64 `bson:"bytes-read-iteration"`
	FlushBytesWritten      float64 `bson:"flush-bytes-written"`
	CompactionBytesRead    float64 `bson:"compaction-bytes-read"`
	CompactionBytesWritten float64 `bson:"compaction-bytes-written"`
}

type RocksDbStats struct {
	NumImmutableMemTable       string                `bson:"num-immutable-mem-table"`
	MemTableFlushPending       string                `bson:"mem-table-flush-pending"`
	CompactionPending          string                `bson:"compaction-pending"`
	BackgroundErrors           string                `bson:"background-errors"`
	CurSizeMemTableActive      string                `bson:"cur-size-active-mem-table"`
	CurSizeAllMemTables        string                `bson:"cur-size-all-mem-tables"`
	NumEntriesMemTableActive   string                `bson:"num-entries-active-mem-table"`
	NumEntriesImmMemTables     string                `bson:"num-entries-imm-mem-tables"`
	EstimateTableReadersMem    string                `bson:"estimate-table-readers-mem"`
	NumSnapshots               string                `bson:"num-snapshots"`
	OldestSnapshotTime         string                `bson:"oldest-snapshot-time"`
	NumLiveVersions            string                `bson:"num-live-versions"`
	BlockCacheUsage            string                `bson:"block-cache-usage"`
	TotalLiveRecoveryUnits     float64               `bson:"total-live-recovery-units"`
	TransactionEngineKeys      float64               `bson:"transaction-engine-keys"`
	TransactionEngineSnapshots float64               `bson:"transaction-engine-snapshots"`
	Stats                      []string              `bson:"stats"`
	ThreadStatus               []string              `bson:"thread-status"`
	Counters                   *RocksDbStatsCounters `bson:"counters,omitempty"`
}

type RocksDbLevelStatsFiles struct {
	Num         float64
	CompThreads float64
}

type RocksDbLevelStats struct {
	Level    string
	Files    *RocksDbLevelStatsFiles
	Score    float64
	SizeMB   float64
	ReadGB   float64
	RnGB     float64
	Rnp1GB   float64
	WriteGB  float64
	WnewGB   float64
	MovedGB  float64
	WAmp     float64
	RdMBPSec float64
	WrMBPSec float64
	CompSec  float64
	CompCnt  float64
	AvgSec   float64
	KeyIn    float64
	KeyDrop  float64
}

// rocksdb time-format string parser: returns float64 of seconds:
func ParseTime(str string) float64 {
	time_str := strings.Split(str, " ")[0]
	time_split := strings.Split(time_str, ":")
	seconds_hour, err := strconv.ParseFloat(time_split[0], 64)
	seconds_min, err := strconv.ParseFloat(time_split[1], 64)
	seconds, err := strconv.ParseFloat(time_split[2], 64)
	if err != nil {
		return float64(-1)
	}
	return (seconds_hour * 3600) + (seconds_min * 60) + seconds
}

// rocksdb metric string parser: converts string-numbers to float64s and parses metric units (MB, KB, etc):
func ParseStr(str string) float64 {
	var multiply float64 = 1
	var str_remove string = ""
	if strings.Contains(str, " KB") || strings.HasSuffix(str, "KB") {
		multiply = kilobyte
		str_remove = "KB"
	} else if strings.Contains(str, " MB") || strings.HasSuffix(str, "MB") {
		multiply = megabyte
		str_remove = "MB"
	} else if strings.Contains(str, " GB") || strings.HasSuffix(str, "GB") {
		multiply = gigabyte
		str_remove = "GB"
	} else if strings.Contains(str, " TB") || strings.HasSuffix(str, "TB") {
		multiply = terabyte
		str_remove = "TB"
	} else if strings.Contains(str, " PB") || strings.HasSuffix(str, "PB") {
		multiply = petabyte
		str_remove = "PB"
	} else if strings.Contains(str, " B") || strings.HasSuffix(str, "B") {
		str_remove = "B"
	} else if strings.HasSuffix(str, "H:M:S") {
		return ParseTime(str)
	} else if strings.Contains(str, "K") {
		first_field := strings.Split(str, " ")[0]
		if strings.HasSuffix(first_field, "K") {
			multiply = thousand
			str_remove = "K"
		}
	} else if strings.Contains(str, "M") {
		first_field := strings.Split(str, " ")[0]
		if strings.HasSuffix(first_field, "M") {
			str_remove = "M"
			multiply = million
		}
	} else if strings.Contains(str, "B") {
		first_field := strings.Split(str, " ")[0]
		if strings.HasSuffix(first_field, "B") {
			str_remove = "B"
			multiply = billion
		}
	} else if strings.Contains(str, "T") {
		first_field := strings.Split(str, " ")[0]
		if strings.HasSuffix(first_field, "T") {
			str_remove = "T"
			multiply = trillion
		}
	}

	if str_remove != "" {
		str = strings.Replace(str, str_remove, "", 1)
	}

	// use the first thing that is a parseable number:
	for _, word := range strings.Split(str, " ") {
		float, err := strconv.ParseFloat(word, 64)
		if err == nil {
			return float * multiply
		}
	}

	return float64(-1)
}

// SplitByWs splits strings with multi-whitespace delimeters into a slice:
func SplitByWs(str string) []string {
	var fields []string
	for _, field := range strings.Split(str, " ") {
		if field != "" && field != " " {
			fields = append(fields, field)
		}
	}
	return fields
}

func ProcessLevelStatsLineFiles(str string) *RocksDbLevelStatsFiles {
	split := strings.Split(str, "/")
	numFiles, err := strconv.ParseFloat(split[0], 64)
	compThreads, err := strconv.ParseFloat(split[1], 64)
	if err != nil {
		return &RocksDbLevelStatsFiles{}
	}
	return &RocksDbLevelStatsFiles{
		Num:         numFiles,
		CompThreads: compThreads,
	}
}

func ProcessLevelStatsLine(line string) *RocksDbLevelStats {
	var stats *RocksDbLevelStats
	if strings.HasPrefix(line, " ") {
		fields := SplitByWs(line)
		stats = &RocksDbLevelStats{
			Level:    fields[0],
			Files:    ProcessLevelStatsLineFiles(fields[1]),
			SizeMB:   ParseStr(fields[2]),
			Score:    ParseStr(fields[3]),
			ReadGB:   ParseStr(fields[4]),
			RnGB:     ParseStr(fields[5]),
			Rnp1GB:   ParseStr(fields[6]),
			WriteGB:  ParseStr(fields[7]),
			WnewGB:   ParseStr(fields[8]),
			MovedGB:  ParseStr(fields[9]),
			WAmp:     ParseStr(fields[10]),
			RdMBPSec: ParseStr(fields[11]),
			WrMBPSec: ParseStr(fields[12]),
			CompSec:  ParseStr(fields[13]),
			CompCnt:  ParseStr(fields[14]),
			AvgSec:   ParseStr(fields[15]),
			KeyIn:    ParseStr(fields[16]),
			KeyDrop:  ParseStr(fields[17]),
		}
	}
	return stats
}

func (stats *RocksDbStats) GetStatsSection(section_prefix string) []string {
	var lines []string
	var is_section bool
	for _, line := range stats.Stats {
		if is_section {
			if line == "" || strings.HasPrefix(line, "** ") && strings.HasSuffix(line, " **") {
				break
			} else if line != "" {
				lines = append(lines, line)
			}
		} else if strings.HasPrefix(line, section_prefix) {
			is_section = true
		}
	}
	return lines
}

func (stats *RocksDbStats) GetStatsLine(section_prefix string, line_prefix string) []string {
	var fields []string
	for _, line := range stats.GetStatsSection(section_prefix) {
		if strings.HasPrefix(line, line_prefix) {
			line = strings.Replace(line, line_prefix, "", 1)
			if strings.Contains(line, ", ") {
				fields = strings.Split(line, ", ")
			} else {
				fields = SplitByWs(line)
			}
		}
	}
	return fields
}

func (stats *RocksDbStats) GetStatsLineField(section_prefix string, line_prefix string, idx int) float64 {
	var field float64 = -1
	stats_line := stats.GetStatsLine(section_prefix, line_prefix)
	if len(stats_line) > idx {
		field = ParseStr(stats_line[idx])
	}
	return field
}

// ProcessLevelStats counts process level stats metrics.
func (stats *RocksDbStats) ProcessLevelStats(ch chan<- prometheus.Metric) {
	var levels []*RocksDbLevelStats
	var is_section bool
	for _, line := range stats.Stats {
		if is_section {
			if strings.HasPrefix(line, " Int") {
				break
			} else if line != "" {
				levels = append(levels, ProcessLevelStatsLine(line))
			}
		} else if strings.HasPrefix(line, "------") {
			is_section = true
		}
	}
	for _, level := range levels {
		levelName := level.Level
		if levelName == "Sum" {
			levelName = "total"
		}
		if levelName != "L0" {
			ch <- prometheus.MustNewConstMetric(rocksDbCompactionBytesDesc, prometheus.CounterValue, level.ReadGB*gigabyte, levelName, "read")
			ch <- prometheus.MustNewConstMetric(rocksDbCompactionBytesDesc, prometheus.CounterValue, level.RnGB*gigabyte, levelName, "read_n")
			ch <- prometheus.MustNewConstMetric(rocksDbCompactionBytesDesc, prometheus.CounterValue, level.Rnp1GB*gigabyte, levelName, "read_np1")
			ch <- prometheus.MustNewConstMetric(rocksDbCompactionBytesDesc, prometheus.CounterValue, level.MovedGB*gigabyte, levelName, "moved")

			rocksDbCompactionBytesPerSec.With(prometheus.Labels{"level": levelName, "type": "read"}).Set(level.RdMBPSec * megabyte)
			rocksDbCompactionWriteAmplification.WithLabelValues(levelName).Set(level.WAmp)
		}
		rocksDbLevelScore.WithLabelValues(levelName).Set(level.Score)
		rocksDbLevelFiles.WithLabelValues(levelName).Set(level.Files.Num)
		rocksDbCompactionThreads.WithLabelValues(levelName).Set(level.Files.CompThreads)
		rocksDbLevelSizeBytes.WithLabelValues(levelName).Set(level.SizeMB * megabyte)

		ch <- prometheus.MustNewConstMetric(rocksDbCompactionSecondsTotalDesc, prometheus.CounterValue, level.CompSec, levelName)

		rocksDbCompactionAvgSeconds.WithLabelValues(levelName).Set(level.AvgSec)

		ch <- prometheus.MustNewConstMetric(rocksDbCompactionBytesDesc, prometheus.CounterValue, level.WriteGB*gigabyte, levelName, "write")
		ch <- prometheus.MustNewConstMetric(rocksDbCompactionBytesDesc, prometheus.CounterValue, level.WriteGB*gigabyte, levelName, "write_new_np1")

		rocksDbCompactionBytesPerSec.With(prometheus.Labels{"level": levelName, "type": "write"}).Set(level.WrMBPSec * megabyte)
		ch <- prometheus.MustNewConstMetric(rocksDbCompactionsTotalDesc, prometheus.CounterValue, level.CompCnt, levelName, "write")
	}
}

// ProcessStalls counts process stalls metrics.
func (stats *RocksDbStats) ProcessStalls(ch chan<- prometheus.Metric) {
	for _, stall_line := range stats.GetStatsLine("** Compaction Stats [default] **", "Stalls(count): ") {
		stall_split := strings.Split(stall_line, " ")
		if len(stall_split) == 2 {
			stall_type := stall_split[1]
			stall_count := stall_split[0]
			ch <- prometheus.MustNewConstMetric(rocksDbStallsDesc, prometheus.CounterValue, ParseStr(stall_count), stall_type)
		}
	}
}

// ProcessReadLatencyStats counts process read latency stats metrics.
func (stats *RocksDbStats) ProcessReadLatencyStats(ch chan<- prometheus.Metric) {
	for _, level_num := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		level := "L" + level_num
		section := "** Level " + level_num + " read latency histogram (micros):"
		if len(stats.GetStatsSection(section)) > 0 {
			ch <- prometheus.MustNewConstMetric(rocksDbReadOpsDesc, prometheus.CounterValue, stats.GetStatsLineField(section, "Count: ", 0), level)

			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "avg"}).Set(stats.GetStatsLineField(section, "Count: ", 2))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "stddev"}).Set(stats.GetStatsLineField(section, "Count: ", 4))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "min"}).Set(stats.GetStatsLineField(section, "Min: ", 0))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "median"}).Set(stats.GetStatsLineField(section, "Min: ", 2))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "max"}).Set(stats.GetStatsLineField(section, "Min: ", 4))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "P50"}).Set(stats.GetStatsLineField(section, "Percentiles: ", 1))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "P75"}).Set(stats.GetStatsLineField(section, "Percentiles: ", 3))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "P99"}).Set(stats.GetStatsLineField(section, "Percentiles: ", 5))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "P99.9"}).Set(stats.GetStatsLineField(section, "Percentiles: ", 7))
			rocksDbReadLatencyMicros.With(prometheus.Labels{"level": level, "type": "P99.99"}).Set(stats.GetStatsLineField(section, "Percentiles: ", 9))
		}
	}
}

func (stats *RocksDbStatsCounters) Describe(ch chan<- *prometheus.Desc) {
	ch <- rocksDbBlockCacheHitsDesc
	ch <- rocksDbBlockCacheMissesDesc
	ch <- rocksDbKeysDesc
	ch <- rocksDbSeeksDesc
	ch <- rocksDbIterationsDesc
	ch <- rocksDbBloomFilterUsefulDesc
	ch <- rocksDbBytesWrittenDesc
	ch <- rocksDbBytesReadDesc
}

func (stats *RocksDbStatsCounters) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(rocksDbBlockCacheHitsDesc, prometheus.CounterValue, stats.BlockCacheHits)
	ch <- prometheus.MustNewConstMetric(rocksDbBlockCacheMissesDesc, prometheus.CounterValue, stats.BlockCacheMisses)
	ch <- prometheus.MustNewConstMetric(rocksDbKeysDesc, prometheus.CounterValue, stats.NumKeysWritten, "written")
	ch <- prometheus.MustNewConstMetric(rocksDbKeysDesc, prometheus.CounterValue, stats.NumKeysRead, "read")

	ch <- prometheus.MustNewConstMetric(rocksDbSeeksDesc, prometheus.CounterValue, stats.NumSeeks)

	ch <- prometheus.MustNewConstMetric(rocksDbIterationsDesc, prometheus.CounterValue, stats.NumForwardIter, "forward")
	ch <- prometheus.MustNewConstMetric(rocksDbIterationsDesc, prometheus.CounterValue, stats.NumBackwardIter, "backward")

	ch <- prometheus.MustNewConstMetric(rocksDbBloomFilterUsefulDesc, prometheus.CounterValue, stats.BloomFilterUseful)

	ch <- prometheus.MustNewConstMetric(rocksDbBytesWrittenDesc, prometheus.CounterValue, stats.BytesWritten, "total")
	ch <- prometheus.MustNewConstMetric(rocksDbBytesWrittenDesc, prometheus.CounterValue, stats.FlushBytesWritten, "flush")
	ch <- prometheus.MustNewConstMetric(rocksDbBytesWrittenDesc, prometheus.CounterValue, stats.CompactionBytesWritten, "compaction")

	ch <- prometheus.MustNewConstMetric(rocksDbBytesReadDesc, prometheus.CounterValue, stats.BytesReadPointLookup, "point_lookup")
	ch <- prometheus.MustNewConstMetric(rocksDbBytesReadDesc, prometheus.CounterValue, stats.BytesReadIteration, "iteration")
	ch <- prometheus.MustNewConstMetric(rocksDbBytesReadDesc, prometheus.CounterValue, stats.CompactionBytesRead, "compation")
}

func (stats *RocksDbStats) Describe(ch chan<- *prometheus.Desc) {
	rocksDbWritesPerBatch.Describe(ch)
	rocksDbWritesPerSec.Describe(ch)
	rocksDbWALBytesPerSecs.Describe(ch)
	rocksDbWALWritesPerSync.Describe(ch)
	rocksDbStallPercent.Describe(ch)
	ch <- rocksDbStalledSecsDesc
	rocksDbLevelFiles.Describe(ch)
	rocksDbCompactionThreads.Describe(ch)
	rocksDbLevelSizeBytes.Describe(ch)
	rocksDbLevelScore.Describe(ch)
	ch <- rocksDbCompactionBytesDesc
	rocksDbCompactionBytesPerSec.Describe(ch)
	rocksDbCompactionWriteAmplification.Describe(ch)
	ch <- rocksDbCompactionSecondsTotalDesc
	rocksDbCompactionAvgSeconds.Describe(ch)
	ch <- rocksDbCompactionsTotalDesc
	rocksDbNumImmutableMemTable.Describe(ch)
	rocksDbMemTableFlushPending.Describe(ch)
	rocksDbCompactionPending.Describe(ch)
	rocksDbBackgroundErrors.Describe(ch)
	rocksDbMemTableBytes.Describe(ch)
	rocksDbMemtableEntries.Describe(ch)
	rocksDbEstimateTableReadersMem.Describe(ch)
	rocksDbNumSnapshots.Describe(ch)
	rocksDbOldestSnapshotTimestamp.Describe(ch)
	rocksDbNumLiveVersions.Describe(ch)
	rocksDbBlockCacheUsage.Describe(ch)
	rocksDbTotalLiveRecoveryUnits.Describe(ch)
	rocksDbTransactionEngineKeys.Describe(ch)
	rocksDbTransactionEngineSnapshots.Describe(ch)

	// optional RocksDB counters
	if stats.Counters != nil {
		stats.Counters.Describe(ch)

		// read latency stats get added to 'stats' when in counter-mode
		ch <- rocksDbReadOpsDesc
		rocksDbReadLatencyMicros.Describe(ch)
	}
}

func (stats *RocksDbStats) Export(ch chan<- prometheus.Metric) {
	// cumulative stats from db.serverStatus().rocksdb.stats (parsed):
	rocksDbWritesPerBatch.Set(stats.GetStatsLineField("** DB Stats **", "Cumulative writes: ", 4))
	rocksDbWritesPerSec.Set(stats.GetStatsLineField("** DB Stats **", "Cumulative writes: ", 5))
	rocksDbWALBytesPerSecs.Set(stats.GetStatsLineField("** DB Stats **", "Cumulative WAL: ", 4))
	rocksDbWALWritesPerSync.Set(stats.GetStatsLineField("** DB Stats **", "Cumulative WAL: ", 2))
	ch <- prometheus.MustNewConstMetric(rocksDbStalledSecsDesc, prometheus.CounterValue, stats.GetStatsLineField("** DB Stats **", "Cumulative stall: ", 0))

	rocksDbStallPercent.Set(stats.GetStatsLineField("** DB Stats **", "Cumulative stall: ", 1))

	// stats from db.serverStatus().rocksdb (parsed):
	rocksDbNumImmutableMemTable.Set(ParseStr(stats.NumImmutableMemTable))
	rocksDbMemTableFlushPending.Set(ParseStr(stats.MemTableFlushPending))
	rocksDbCompactionPending.Set(ParseStr(stats.CompactionPending))
	rocksDbBackgroundErrors.Set(ParseStr(stats.BackgroundErrors))
	rocksDbMemtableEntries.WithLabelValues("active").Set(ParseStr(stats.NumEntriesMemTableActive))
	rocksDbMemtableEntries.WithLabelValues("immutable").Set(ParseStr(stats.NumEntriesImmMemTables))
	rocksDbNumSnapshots.Set(ParseStr(stats.NumSnapshots))
	rocksDbOldestSnapshotTimestamp.Set(ParseStr(stats.OldestSnapshotTime))
	rocksDbNumLiveVersions.Set(ParseStr(stats.NumLiveVersions))
	rocksDbBlockCacheUsage.Set(ParseStr(stats.BlockCacheUsage))
	rocksDbEstimateTableReadersMem.Set(ParseStr(stats.EstimateTableReadersMem))
	rocksDbMemTableBytes.WithLabelValues("active").Set(ParseStr(stats.CurSizeMemTableActive))
	rocksDbMemTableBytes.WithLabelValues("total").Set(ParseStr(stats.CurSizeAllMemTables))

	// stats from db.serverStatus().rocksdb (unparsed - somehow these aren't real types!):
	rocksDbTotalLiveRecoveryUnits.Set(stats.TotalLiveRecoveryUnits)
	rocksDbTransactionEngineKeys.Set(stats.TransactionEngineKeys)
	rocksDbTransactionEngineSnapshots.Set(stats.TransactionEngineSnapshots)

	// process per-level stats in to vectors:
	stats.ProcessLevelStats(ch)

	// process stall counts into a vector:
	stats.ProcessStalls(ch)

	rocksDbWritesPerBatch.Collect(ch)
	rocksDbWritesPerSec.Collect(ch)
	rocksDbWALBytesPerSecs.Collect(ch)
	rocksDbWALWritesPerSync.Collect(ch)
	rocksDbStallPercent.Collect(ch)
	rocksDbLevelFiles.Collect(ch)
	rocksDbCompactionThreads.Collect(ch)
	rocksDbLevelSizeBytes.Collect(ch)
	rocksDbLevelScore.Collect(ch)
	rocksDbCompactionBytesPerSec.Collect(ch)
	rocksDbCompactionWriteAmplification.Collect(ch)
	rocksDbCompactionAvgSeconds.Collect(ch)
	rocksDbNumImmutableMemTable.Collect(ch)
	rocksDbMemTableFlushPending.Collect(ch)
	rocksDbCompactionPending.Collect(ch)
	rocksDbBackgroundErrors.Collect(ch)
	rocksDbMemtableEntries.Collect(ch)
	rocksDbNumSnapshots.Collect(ch)
	rocksDbOldestSnapshotTimestamp.Collect(ch)
	rocksDbNumLiveVersions.Collect(ch)
	rocksDbTotalLiveRecoveryUnits.Collect(ch)
	rocksDbTransactionEngineKeys.Collect(ch)
	rocksDbTransactionEngineSnapshots.Collect(ch)
	rocksDbMemTableBytes.Collect(ch)
	rocksDbEstimateTableReadersMem.Collect(ch)
	rocksDbBlockCacheUsage.Collect(ch)

	// optional RocksDB counters
	if stats.Counters != nil {
		stats.Counters.Export(ch)

		// read latency stats get added to 'stats' when in counter-mode
		stats.ProcessReadLatencyStats(ch)
		rocksDbReadLatencyMicros.Collect(ch)
	}
}
