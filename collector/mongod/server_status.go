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
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	commoncollector "github.com/percona/mongodb_exporter/collector/common"
)

// ServerStatus keeps the data returned by the serverStatus() method.
type ServerStatus struct {
	commoncollector.ServerStatus `bson:",inline"`

	Dur *DurStats `bson:"dur"`

	BackgroundFlushing *FlushStats `bson:"backgroundFlushing"`

	GlobalLock *GlobalLockStats `bson:"globalLock"`

	IndexCounter *IndexCounterStats `bson:"indexCounters"`

	Locks LockStatsMap `bson:"locks,omitempty"`

	OpLatencies *OpLatenciesStat `bson:"opLatencies"`
	Metrics     *MetricsStats    `bson:"metrics"`

	StorageEngine *StorageEngineStats `bson:"storageEngine"`
	InMemory      *WiredTigerStats    `bson:"inMemory"`
	RocksDb       *RocksDbStats       `bson:"rocksdb"`
	WiredTiger    *WiredTigerStats    `bson:"wiredTiger"`

	Ok float64 `bson:"ok"`
}

// Export exports the server status to be consumed by prometheus.
func (status *ServerStatus) Export(ch chan<- prometheus.Metric) {
	status.ServerStatus.Export(ch)
	if status.Dur != nil {
		status.Dur.Export(ch)
	}
	if status.BackgroundFlushing != nil {
		status.BackgroundFlushing.Export(ch)
	}
	if status.GlobalLock != nil {
		status.GlobalLock.Export(ch)
	}
	if status.IndexCounter != nil {
		status.IndexCounter.Export(ch)
	}
	if status.OpLatencies != nil {
		status.OpLatencies.Export(ch)
	}
	if status.Locks != nil {
		status.Locks.Export(ch)
	}
	if status.Metrics != nil {
		status.Metrics.Export(ch)
	}
	if status.InMemory != nil {
		status.InMemory.Export(ch)
	}
	if status.RocksDb != nil {
		status.RocksDb.Export(ch)
	}
	if status.WiredTiger != nil {
		status.WiredTiger.Export(ch)
	}
	// If db.serverStatus().storageEngine does not exist (3.0+ only) and status.BackgroundFlushing does (MMAPv1 only), default to mmapv1
	// https://docs.mongodb.com/v3.0/reference/command/serverStatus/#storageengine
	if status.StorageEngine == nil && status.BackgroundFlushing != nil {
		status.StorageEngine = &StorageEngineStats{
			Name: "mmapv1",
		}
	}
	if status.StorageEngine != nil {
		status.StorageEngine.Export(ch)
	}
}

// Describe describes the server status for prometheus.
func (status *ServerStatus) Describe(ch chan<- *prometheus.Desc) {
	status.ServerStatus.Describe(ch)
	if status.Dur != nil {
		status.Dur.Describe(ch)
	}
	if status.BackgroundFlushing != nil {
		status.BackgroundFlushing.Describe(ch)
	}
	if status.GlobalLock != nil {
		status.GlobalLock.Describe(ch)
	}
	if status.IndexCounter != nil {
		status.IndexCounter.Describe(ch)
	}
	if status.OpLatencies != nil {
		status.OpLatencies.Describe(ch)
	}
	if status.Opcounters != nil {
		status.Opcounters.Describe(ch)
	}
	if status.OpcountersRepl != nil {
		status.OpcountersRepl.Describe(ch)
	}
	if status.Locks != nil {
		status.Locks.Describe(ch)
	}
	if status.Metrics != nil {
		status.Metrics.Describe(ch)
	}
	if status.StorageEngine != nil {
		status.StorageEngine.Describe(ch)
	}
	if status.InMemory != nil {
		status.InMemory.Describe(ch)
	}
	if status.RocksDb != nil {
		status.RocksDb.Describe(ch)
	}
	if status.WiredTiger != nil {
		status.WiredTiger.Describe(ch)
	}
}

// GetServerStatus returns the server status info.
func GetServerStatus(client *mongo.Client) *ServerStatus {
	result := &ServerStatus{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{
		{Key: "serverStatus", Value: 1},
		{Key: "recordStats", Value: 0},
		{Key: "opLatencies", Value: bson.M{"histograms": true}},
	}).Decode(result)
	if err != nil {
		log.Errorf("Failed to get server status: %s", err)
		return nil
	}

	return result
}
