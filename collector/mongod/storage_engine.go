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
	storageEngineDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "storage_engine"),
		"The storage engine used by the MongoDB instance",
		[]string{"engine"},
		nil,
	)
)

// StorageEngineStats
type StorageEngineStats struct {
	Name string `bson:"name"`
}

// Export exports the data to prometheus.
func (stats *StorageEngineStats) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(storageEngineDesc, prometheus.CounterValue, 1, stats.Name)
}

// Describe describes the metrics for prometheus
func (stats *StorageEngineStats) Describe(ch chan<- *prometheus.Desc) {
	ch <- storageEngineDesc
}
