package collector_mongod

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	durabilityCommits = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "durability_commits",
		Help:      "Durability commits",
	}, []string{"state"})
)
var (
	durabilityJournaledMegabytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "durability",
		Name:      "journaled_megabytes",
		Help:      "The journaledMB provides the amount of data in megabytes (MB) written to journal during the last journal group commit interval",
	})
	durabilityWriteToDataFilesMegabytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "durability",
		Name:      "write_to_data_files_megabytes",
		Help:      "The writeToDataFilesMB provides the amount of data in megabytes (MB) written from journal to the data files during the last journal group commit interval",
	})
	durabilityCompression = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "durability",
		Name:      "compression",
		Help:      "The compression represents the compression ratio of the data written to the journal: ( journaled_size_of_data / uncompressed_size_of_data )",
	})
	durabilityEarlyCommits = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: Namespace,
		Subsystem: "durability",
		Name:      "early_commits",
		Help:      "The earlyCommits value reflects the number of times MongoDB requested a commit before the scheduled journal group commit interval. Use this value to ensure that your journal group commit interval is not too long for your deployment",
	})
)
var (
	durabilityTimeMilliseconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: Namespace,
		Name:      "durability_time_milliseconds",
		Help:      "Summary of times spent during the journaling process.",
	}, []string{"stage"})
)

// DurTiming is the information about durability returned from the server.
type DurTiming struct {
	Dt               float64 `bson:"dt"`
	PrepLogBuffer    float64 `bson:"prepLogBuffer"`
	WriteToJournal   float64 `bson:"writeToJournal"`
	WriteToDataFiles float64 `bson:"writeToDataFiles"`
	RemapPrivateView float64 `bson:"remapPrivateView"`
}

// Export exports the data for the prometheus server.
func (durTiming *DurTiming) Export(ch chan<- prometheus.Metric) {
	durabilityTimeMilliseconds.WithLabelValues("dt").Observe(durTiming.Dt)
	durabilityTimeMilliseconds.WithLabelValues("prep_log_buffer").Observe(durTiming.PrepLogBuffer)
	durabilityTimeMilliseconds.WithLabelValues("write_to_journal").Observe(durTiming.WriteToJournal)
	durabilityTimeMilliseconds.WithLabelValues("write_to_data_files").Observe(durTiming.WriteToDataFiles)
	durabilityTimeMilliseconds.WithLabelValues("remap_private_view").Observe(durTiming.RemapPrivateView)
	durabilityTimeMilliseconds.Collect(ch)
}

// DurStats are the stats related to durability.
type DurStats struct {
	Commits            float64   `bson:"commits"`
	JournaledMB        float64   `bson:"journaledMB"`
	WriteToDataFilesMB float64   `bson:"writeToDataFilesMB"`
	Compression        float64   `bson:"compression"`
	CommitsInWriteLock float64   `bson:"commitsInWriteLock"`
	EarlyCommits       float64   `bson:"earlyCommits"`
	TimeMs             DurTiming `bson:"timeMs"`
}

// Export export the durability stats for the prometheus server.
func (durStats *DurStats) Export(ch chan<- prometheus.Metric) {
	durabilityCommits.WithLabelValues("written").Set(durStats.Commits)
	durabilityCommits.WithLabelValues("in_write_lock").Set(durStats.CommitsInWriteLock)

	durabilityJournaledMegabytes.Set(durStats.JournaledMB)
	durabilityWriteToDataFilesMegabytes.Set(durStats.WriteToDataFilesMB)
	durabilityCompression.Set(durStats.Compression)
	durabilityEarlyCommits.Observe(durStats.EarlyCommits)

	durStats.TimeMs.Export(ch)

	durStats.Collect(ch)
}

// Collect collects the metrics for prometheus
func (durStats *DurStats) Collect(ch chan<- prometheus.Metric) {
	durabilityCommits.Collect(ch)
	durabilityJournaledMegabytes.Collect(ch)
	durabilityWriteToDataFilesMegabytes.Collect(ch)
	durabilityCompression.Collect(ch)
	durabilityEarlyCommits.Collect(ch)
}

// Describe describes the metrics for prometheus
func (durStats *DurStats) Describe(ch chan<- *prometheus.Desc) {
	durabilityCommits.Describe(ch)
	durabilityJournaledMegabytes.Describe(ch)
	durabilityWriteToDataFilesMegabytes.Describe(ch)
	durabilityCompression.Describe(ch)
	durabilityEarlyCommits.Describe(ch)
	durabilityTimeMilliseconds.Describe(ch)
}
