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
	connections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "connections",
		Help:      "The connections sub document data regarding the current status of incoming connections and availability of the database server. Use these values to assess the current load and capacity requirements of the server",
	}, []string{"state"})
)
var (
	connectionsMetricsCreatedTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "connections_metrics", "created_total"),
		"totalCreated provides a count of all incoming connections created to the server. This number includes connections that have since closed",
		nil,
		nil,
	)
)

// ConnectionStats are connections metrics
type ConnectionStats struct {
	Current      float64  `bson:"current"`
	Available    float64  `bson:"available"`
	TotalCreated float64  `bson:"totalCreated"`
	Active       *float64 `bson:"active,omitempty"`
}

// Export exports the data to prometheus.
func (connectionStats *ConnectionStats) Export(ch chan<- prometheus.Metric) {
	connections.WithLabelValues("current").Set(connectionStats.Current)
	connections.WithLabelValues("available").Set(connectionStats.Available)
	// new in Mongo 4.0.7
	if connectionStats.Active != nil {
		connections.WithLabelValues("active").Set(*connectionStats.Active)
	}
	connections.Collect(ch)

	ch <- prometheus.MustNewConstMetric(connectionsMetricsCreatedTotalDesc, prometheus.CounterValue, connectionStats.TotalCreated)
}

// Describe describes the metrics for prometheus
func (connectionStats *ConnectionStats) Describe(ch chan<- *prometheus.Desc) {
	connections.Describe(ch)
	ch <- connectionsMetricsCreatedTotalDesc
}
