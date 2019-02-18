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

package collector_common

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
	instanceUptimeSeconds = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "instance",
		Name:      "uptime_seconds",
		Help:      "The value of the uptime field corresponds to the number of seconds that the mongos or mongod process has been active.",
	})
	instanceUptimeEstimateSeconds = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "instance",
		Name:      "uptime_estimate_seconds",
		Help:      "uptimeEstimate provides the uptime as calculated from MongoDB's internal course-grained time keeping system.",
	})
	instanceLocalTime = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "instance",
		Name:      "local_time",
		Help:      "The localTime value is the current time, according to the server, in UTC specified in an ISODate format.",
	})
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
}

// Export exports the server status to be consumed by prometheus.
func (status *ServerStatus) Export(ch chan<- prometheus.Metric) {
	versionInfo.WithLabelValues(status.Version).Set(1)
	instanceUptimeSeconds.Set(status.Uptime)
	instanceUptimeEstimateSeconds.Set(status.Uptime)
	instanceLocalTime.Set(float64(status.LocalTime.Unix()))
	versionInfo.Collect(ch)
	instanceUptimeSeconds.Collect(ch)
	instanceUptimeEstimateSeconds.Collect(ch)
	instanceLocalTime.Collect(ch)

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
}

// Describe describes the server status for prometheus.
func (status *ServerStatus) Describe(ch chan<- *prometheus.Desc) {
	instanceUptimeSeconds.Describe(ch)
	instanceUptimeEstimateSeconds.Describe(ch)
	instanceLocalTime.Describe(ch)

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
}
