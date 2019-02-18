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

package mongos

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/percona/mongodb_exporter/collector/common"
)

// ServerStatus keeps the data returned by the serverStatus() method.
type ServerStatus struct {
	collector_common.ServerStatus `bson:",inline"`

	Metrics *MetricsStats `bson:"metrics"`
}

// Export exports the server status to be consumed by prometheus.
func (status *ServerStatus) Export(ch chan<- prometheus.Metric) {
	status.ServerStatus.Export(ch)
	if status.Metrics != nil {
		status.Metrics.Export(ch)
	}
}

// Describe describes the server status for prometheus.
func (status *ServerStatus) Describe(ch chan<- *prometheus.Desc) {
	status.ServerStatus.Describe(ch)
	if status.Metrics != nil {
		status.Metrics.Describe(ch)
	}
}

// GetServerStatus returns the server status info.
func GetServerStatus(session *mgo.Session) *ServerStatus {
	result := &ServerStatus{}
	err := session.DB("admin").Run(bson.D{{"serverStatus", 1}, {"recordStats", 0}}, result)
	if err != nil {
		log.Errorf("Failed to get server status: %s", err)
		return nil
	}

	return result
}
