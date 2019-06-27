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
	opLatenciesHistogram    *prometheus.HistogramVec
	prevLatencyStatReads    *LatencyStat
	prevLatencyStatWrites   *LatencyStat
	prevLatencyStatCommands *LatencyStat
)

func InitOpLatenciesMetrics(start, width float64, count int) {
	opLatenciesHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "op_latency_microseconds",
		Help:    "Operation (read/write/command) latencies histogram from mongod",
		Buckets: prometheus.LinearBuckets(start, width, count),
	}, []string{"type"})
}

// HistBucket describes a item of op latencies histogram
type HistBucket struct {
	Micros int64   `bson:"micros"`
	Count  float64 `bson:"count"`
}

// LatencyStat describes op latencies statistic
type LatencyStat struct {
	Histogram []HistBucket `bson:"histogram"`
	Latency   float64      `bson:"latency"`
	Ops       float64      `bson:"ops"`
}

// Update each metric
func (ls *LatencyStat) Update(op string, prevLs *LatencyStat) {
	if ls.Histogram != nil {
		for _, bucket := range ls.Histogram {
			loopUpperBound := clipObservationCount(prevLs, bucket.Micros, int64(bucket.Count))
			// Prometheus histogram typically will have much fewer binds and on
			// boundaries different from mongodb. For best approximation compute midpoint
			// of histogram bin from mongodb
			observationMicros := histMicrosEdgeToMidpoint(bucket.Micros)
			for i := int64(0); i < loopUpperBound; i++ {
				opLatenciesHistogram.WithLabelValues(op).Observe(observationMicros)
			}
		}
	}
}

//  Returns true if input is a power of 2
func isPow2(arg uint64) bool {
	return (arg > 0) && ((arg & (arg - 1)) == 0)
}

//  Documentation of histogram bins
//  https://docs.mongodb.com/manual/reference/operator/aggregation/collStats/#latencystats-document
//  "An array of embedded documents, each representing a latency range. Each
//  document covers twice the previous document’s range. For upper values between
//  2048 microseconds and roughly 1 second, the histogram includes half-steps."
//
//  My interpretation is the histogram bin edges are predefined: 1, 2, 4, 8 ... 2048, (2048+2048/2), 4096, (4096+4096/2), ...
//  Or another way									           : 1, 2, 4, 8 ... 2048, (4096-4096/4), 4096, (8192-8192/4), ...
//
//  Input that is not exactly one of the documented histogram boundaries is not expected and will lead to incorrect midpoint
func histMicrosEdgeToMidpoint(x int64) float64 {
	if x <= 1 {
		return 0.5
	} else if x <= 2048 {
		// midpoint between (x/2, x]
		// e.g. midpoint between (1024, 2048] where input x := 2048
		// taking the average: (x/2 + x)/2 = 3x/4
		return 3.0 * float64(x) / 4.0
	} else {
		if isPow2(uint64(x)) {
			// midpoint between (x-x/4 ,x] where x is a power of 2 and not "half step" as described in mongodb docs
			// e.g. midpoint between (3072, 4096] where input x := 4096
			// taking the average: ((x - x/4) + x)/2 = x*7/8
			return float64(x) * 7.0 / 8.0
		} else {
			// midpoint between (y, y+y/2] where x is a "half step" endpoint as described in mongodb docs, and x = y+y/2, for some y which is a power of 2
			// e.g. midpoint between (2048, 3072] where input x := 3072, and y := 2048
			// Expressing interval (y, y+y/2] in terms of x, (2/3*x, x]
			// taking the average: (2/3*x + x)/2 = x*5/6
			return float64(x) * 5.0 / 6.0
		}
	}
}

// This function assumes monotonically increasing HistBucket.Micros
func clipObservationCount(ls *LatencyStat, targetLatMicros int64, count int64) int64 {
	// No previous observation recorded, observe all
	if ls == nil || len(ls.Histogram) == 0 {
		return count
	}
	for _, bucket := range ls.Histogram {
		if targetLatMicros == bucket.Micros {
			return count - int64(bucket.Count)
		}
	}
	return count
}

// OpLatenciesStat includes reads, writes and commands latency statistic
type OpLatenciesStat struct {
	Reads    *LatencyStat `bson:"reads"`
	Writes   *LatencyStat `bson:"writes"`
	Commands *LatencyStat `bson:"commands"`
}

// Export exports metrics to Prometheus
func (stat *OpLatenciesStat) Export(ch chan<- prometheus.Metric) {
	if stat.Reads != nil {
		stat.Reads.Update("read", prevLatencyStatReads)
		prevLatencyStatReads = stat.Reads
	}
	if stat.Writes != nil {
		stat.Writes.Update("write", prevLatencyStatWrites)
		prevLatencyStatWrites = stat.Writes
	}
	if stat.Commands != nil {
		stat.Commands.Update("command", prevLatencyStatCommands)
		prevLatencyStatCommands = stat.Commands
	}
	opLatenciesHistogram.Collect(ch)
}

// Describe describes the metrics for prometheus
func (stat *OpLatenciesStat) Describe(ch chan<- *prometheus.Desc) {
	opLatenciesHistogram.Describe(ch)
}
