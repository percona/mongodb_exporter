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
	"github.com/prometheus/client_golang/prometheus"
)

var (
	cursorsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "cursors",
		Help:      "The cursors data structure contains data regarding cursor state and use",
	}, []string{"state"})
)

// Cursors are the cursor metrics
type Cursors struct {
	TotalOpen      float64 `bson:"totalOpen"`
	TimeOut        float64 `bson:"timedOut"`
	TotalNoTimeout float64 `bson:"totalNoTimeout"`
	Pinned         float64 `bson:"pinned"`
}

// Export exports the data to prometheus.
func (cursors *Cursors) Export(ch chan<- prometheus.Metric) {
	cursorsGauge.WithLabelValues("total_open").Set(cursors.TotalOpen)
	cursorsGauge.WithLabelValues("timed_out").Set(cursors.TimeOut)
	cursorsGauge.WithLabelValues("total_no_timeout").Set(cursors.TotalNoTimeout)
	cursorsGauge.WithLabelValues("pinned").Set(cursors.Pinned)
	cursorsGauge.Collect(ch)
}

// Describe describes the metrics for prometheus
func (cursors *Cursors) Describe(ch chan<- *prometheus.Desc) {
	cursorsGauge.Describe(ch)
}
