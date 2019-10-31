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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/percona/mongodb_exporter/shared"
)

var (
	oplogDb          = "local"
	oplogCollection  = "oplog.rs"
	oplogStatusCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "replset_oplog",
		Name:      "items_total",
		Help:      "The total number of changes in the oplog",
	})
	oplogStatusHeadTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "replset_oplog",
		Name:      "head_timestamp",
		Help:      "The timestamp of the newest change in the oplog",
	})
	oplogStatusTailTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "replset_oplog",
		Name:      "tail_timestamp",
		Help:      "The timestamp of the oldest change in the oplog",
	})
	oplogStatusSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "replset_oplog",
		Name:      "size_bytes",
		Help:      "Size of oplog in bytes",
	}, []string{"type"})
)

type OplogCollectionStats struct {
	Count       float64 `bson:"count"`
	Size        float64 `bson:"size"`
	StorageSize float64 `bson:"storageSize"`
}

type OplogTimestamps struct {
	Tail float64
	Head float64
}

type OplogStatus struct {
	OplogTimestamps *OplogTimestamps
	CollectionStats *OplogCollectionStats
}

func getOplogTailOrHeadTimestamp(client *mongo.Client, returnHead bool) (float64, error) {
	var result struct {
		Timestamp primitive.Timestamp `bson:"ts"` // See: https://docs.mongodb.com/manual/reference/bson-types/#timestamps
	}

	var sortCond = bson.M{"$natural": 1}
	if returnHead {
		sortCond = bson.M{"$natural": -1}
	}

	opts := options.FindOne().SetComment(shared.GetCallerLocation()).SetSort(sortCond)
	err := client.Database(oplogDb).Collection(oplogCollection).FindOne(context.TODO(), bson.M{}, opts).Decode(&result)
	return float64(result.Timestamp.T), err
}

// GetOplogTimestamps gets oplog timestamps.
func GetOplogTimestamps(client *mongo.Client) (*OplogTimestamps, error) {
	headTs, err := getOplogTailOrHeadTimestamp(client, true)
	if err != nil {
		return nil, err
	}
	tailTs, err := getOplogTailOrHeadTimestamp(client, false)
	if err != nil {
		return nil, err
	}
	oplogTimestamps := &OplogTimestamps{
		Head: headTs,
		Tail: tailTs,
	}
	return oplogTimestamps, err
}

// GetOplogCollectionStats gets oplog connection stats.
func GetOplogCollectionStats(client *mongo.Client) (*OplogCollectionStats, error) {
	results := &OplogCollectionStats{}
	err := client.Database("local").RunCommand(context.TODO(), bson.M{"collStats": "oplog.rs"}).Decode(&results)
	return results, err
}

func (status *OplogStatus) Export(ch chan<- prometheus.Metric) {
	oplogStatusSizeBytes.WithLabelValues("current").Set(0)
	oplogStatusSizeBytes.WithLabelValues("storage").Set(0)
	if status.CollectionStats != nil {
		oplogStatusCount.Set(status.CollectionStats.Count)
		oplogStatusSizeBytes.WithLabelValues("current").Set(status.CollectionStats.Size)
		oplogStatusSizeBytes.WithLabelValues("storage").Set(status.CollectionStats.StorageSize)
	}
	if status.OplogTimestamps != nil {
		oplogStatusHeadTimestamp.Set(status.OplogTimestamps.Head)
		oplogStatusTailTimestamp.Set(status.OplogTimestamps.Tail)
	}

	oplogStatusCount.Collect(ch)
	oplogStatusHeadTimestamp.Collect(ch)
	oplogStatusTailTimestamp.Collect(ch)
	oplogStatusSizeBytes.Collect(ch)
}

func (status *OplogStatus) Describe(ch chan<- *prometheus.Desc) {
	oplogStatusCount.Describe(ch)
	oplogStatusHeadTimestamp.Describe(ch)
	oplogStatusTailTimestamp.Describe(ch)
	oplogStatusSizeBytes.Describe(ch)
}

// GetOplogStatus gets oplog status.
func GetOplogStatus(client *mongo.Client) *OplogStatus {
	collectionStats, err := GetOplogCollectionStats(client)
	if err != nil {
		log.Errorf("Failed to get oplog collection status: %s", err)
	}

	oplogTimestamps, err := GetOplogTimestamps(client)
	if err != nil {
		log.Errorf("Failed to get oplog timestamps status: %s", err)
		return nil
	}

	return &OplogStatus{CollectionStats: collectionStats, OplogTimestamps: oplogTimestamps}
}
