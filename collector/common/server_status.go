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

package common

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	versionInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "version",
		Name:      "info",
		Help:      "Software version information for mongodb process.",
	}, []string{"mongodb"})
	instanceUptimeSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "instance", "uptime_seconds"),
		"The value of the uptime field corresponds to the number of seconds that the mongos or mongod process has been active.",
		nil,
		nil,
	)
	instanceUptimeEstimateSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "instance", "uptime_estimate_seconds"),
		"uptimeEstimate provides the uptime as calculated from MongoDB's internal course-grained time keeping system.",
		nil,
		nil,
	)
	instanceLocalTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "instance", "local_time"),
		"The localTime value is the current time, according to the server, in UTC specified in an ISODate format.",
		nil,
		nil,
	)
)

// ServerStatus keeps the data returned by the serverStatus() method.
type ServerStatus struct {
	Version        string    `bson:"version"`
	Uptime         float64   `bson:"uptime"`
	UptimeEstimate float64   `bson:"uptimeEstimate"`
	LocalTime      time.Time `bson:"localTime"`

	Asserts     *AssertsStats    `bson:"asserts"`
	Connections *ConnectionStats `bson:"connections"`
	Cursors     *Cursors         `bson:"cursors"`
	ExtraInfo   *ExtraInfo       `bson:"extra_info"`
	Mem         *MemStats        `bson:"mem"`
	Network     *NetworkStats    `bson:"network"`

	Opcounters     *OpcountersStats     `bson:"opcounters"`
	OpcountersRepl *OpcountersReplStats `bson:"opcountersRepl"`

	TCMallocStats *TCMallocStats `bson:"tcmalloc"`
}

// Export exports the server status to be consumed by prometheus.
func (status *ServerStatus) Export(ch chan<- prometheus.Metric) {
	versionInfo.WithLabelValues(status.Version).Set(1)
	ch <- prometheus.MustNewConstMetric(instanceUptimeSecondsDesc, prometheus.CounterValue, status.Uptime)
	ch <- prometheus.MustNewConstMetric(instanceUptimeEstimateSecondsDesc, prometheus.CounterValue, status.Uptime)
	ch <- prometheus.MustNewConstMetric(instanceLocalTimeDesc, prometheus.GaugeValue, float64(status.LocalTime.Unix()))
	versionInfo.Collect(ch)

	if status.Asserts != nil {
		status.Asserts.Export(ch)
	}
	if status.Connections != nil {
		status.Connections.Export(ch)
	}
	if status.Cursors != nil {
		status.Cursors.Export(ch)
	}
	if status.ExtraInfo != nil {
		status.ExtraInfo.Export(ch)
	}
	if status.Mem != nil {
		status.Mem.Export(ch)
	}
	if status.Network != nil {
		status.Network.Export(ch)
	}
	if status.Opcounters != nil {
		status.Opcounters.Export(ch)
	}
	if status.OpcountersRepl != nil {
		status.OpcountersRepl.Export(ch)
	}
	if status.TCMallocStats != nil {
		status.TCMallocStats.Export(ch)
	}
}

// Describe describes the server status for prometheus.
func (status *ServerStatus) Describe(ch chan<- *prometheus.Desc) {
	ch <- instanceUptimeSecondsDesc
	ch <- instanceUptimeEstimateSecondsDesc
	ch <- instanceLocalTimeDesc

	if status.Asserts != nil {
		status.Asserts.Describe(ch)
	}
	if status.Connections != nil {
		status.Connections.Describe(ch)
	}
	if status.Cursors != nil {
		status.Cursors.Describe(ch)
	}
	if status.ExtraInfo != nil {
		status.ExtraInfo.Describe(ch)
	}
	if status.Mem != nil {
		status.Mem.Describe(ch)
	}
	if status.Network != nil {
		status.Network.Describe(ch)
	}
	if status.TCMallocStats != nil {
		status.TCMallocStats.Describe(ch)
	}
}
