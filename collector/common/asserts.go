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
	assertsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "asserts_total"),
		"The asserts document reports the number of asserts on the database. While assert errors are typically uncommon, if there are non-zero values for the asserts, you should check the log file for the mongod process for more information. In many cases these errors are trivial, but are worth investigating.",
		[]string{"type"},
		nil,
	)
)

// AssertsStats has the assets metrics
type AssertsStats struct {
	Regular   float64 `bson:"regular"`
	Warning   float64 `bson:"warning"`
	Msg       float64 `bson:"msg"`
	User      float64 `bson:"user"`
	Rollovers float64 `bson:"rollovers"`
}

// Export exports the metrics to prometheus.
func (asserts *AssertsStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(assertsTotalDesc, prometheus.CounterValue, asserts.Regular, "regular")
	ch <- prometheus.MustNewConstMetric(assertsTotalDesc, prometheus.CounterValue, asserts.Warning, "warning")
	ch <- prometheus.MustNewConstMetric(assertsTotalDesc, prometheus.CounterValue, asserts.Msg, "msg")
	ch <- prometheus.MustNewConstMetric(assertsTotalDesc, prometheus.CounterValue, asserts.User, "user")
	ch <- prometheus.MustNewConstMetric(assertsTotalDesc, prometheus.CounterValue, asserts.Rollovers, "rollovers")
}

// Describe describes the metrics for prometheus
func (asserts *AssertsStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- assertsTotalDesc
}
