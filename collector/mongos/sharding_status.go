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
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/percona/mongodb_exporter/shared"
)

var (
	balancerIsEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "balancer_enabled",
		Help:      "Boolean reporting if cluster balancer is enabled (1 = enabled/0 = disabled)",
	})
	balancerChunksBalanced = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "chunks_is_balanced",
		Help:      "Boolean reporting if cluster chunks are evenly balanced across shards (1 = yes/0 = no)",
	})
	mongosUpSecs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "mongos_uptime_seconds",
		Help:      "The uptime of the Mongos processes in seconds",
	}, []string{"name"})
	mongosPing = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "mongos_last_ping_timestamp",
		Help:      "The unix timestamp of the last Mongos ping to the Cluster config servers",
	}, []string{"name"})
	mongosBalancerLockTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "balancer_lock_timestamp",
		Help:      "The unix timestamp of the last update to the Cluster balancer lock",
	}, []string{"name"})
	mongosBalancerLockState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "balancer_lock_state",
		Help:      "The state of the Cluster balancer lock (-1 = none/0 = unlocked/1 = contention/2 = locked)",
	}, []string{"name"})
)

type MongosInfo struct {
	Name         string    `bson:"_id"`
	Ping         time.Time `bson:"ping"`
	Up           float64   `bson:"up"`
	Waiting      bool      `bson:"waiting"`
	MongoVersion string    `bson:"mongoVersion"`
}

type MongosBalancerLock struct {
	State   float64   `bson:"state"`
	Process string    `bson:"process"`
	Who     string    `bson:"who"`
	When    time.Time `bson:"when"`
	Why     string    `bson:"why"`
}

type ShardingStats struct {
	IsBalanced      float64
	BalancerEnabled float64
	Changelog       *ShardingChangelogStats
	Topology        *ShardingTopoStats
	BalancerLock    *MongosBalancerLock
	Mongos          *[]MongosInfo

	Client *mongo.Client
}

// GetMongosInfo gets mongos info.
func GetMongosInfo(client *mongo.Client) *[]MongosInfo {
	mongosInfo := []MongosInfo{}
	opts := options.Find().SetComment(shared.GetCallerLocation())
	c, err := client.Database("config").Collection("mongos").Find(context.TODO(), bson.M{"ping": bson.M{"$gte": time.Now().Add(-10 * time.Minute)}}, opts)
	if err != nil {
		log.Errorf("Failed to execute find query on 'config.mongos': %s.", err)
		return nil
	}
	defer c.Close(context.TODO())

	for c.Next(context.TODO()) {
		i := &MongosInfo{}
		if err := c.Decode(i); err != nil {
			log.Error(err)
			continue
		}
		mongosInfo = append(mongosInfo, *i)
	}

	if err := c.Err(); err != nil {
		log.Error(err)
	}

	return &mongosInfo
}

// GetMongosBalancerLock gets mongos balncer lock.
func GetMongosBalancerLock(client *mongo.Client) *MongosBalancerLock {
	var balancerLock *MongosBalancerLock
	opts := options.FindOne().SetComment(shared.GetCallerLocation())
	r := client.Database("config").Collection("locks").FindOne(context.TODO(), bson.M{"_id": "balancer"}, opts)
	if err := r.Decode(&balancerLock); err != nil {
		log.Errorf("Failed to execute find query on 'config.locks': %s.", err)
	}
	return balancerLock
}

// IsBalancerEnabled check is balancer enabled.
func IsBalancerEnabled(client *mongo.Client) float64 {
	balancerConfig := struct {
		Stopped bool `bson:"stopped"`
	}{}
	opts := options.FindOne().SetComment(shared.GetCallerLocation())
	r := client.Database("config").Collection("settings").FindOne(context.TODO(), bson.M{"_id": "balancer"}, opts)
	if err := r.Decode(&balancerConfig); err != nil {
		return 1
	}
	if balancerConfig.Stopped {
		return 0
	}
	return 1
}

// IsClusterBalanced check is cluster balanced.
func IsClusterBalanced(client *mongo.Client) float64 {
	// Different thresholds based on size
	// http://docs.mongodb.org/manual/core/sharding-internals/#sharding-migration-thresholds
	var threshold float64 = 8
	totalChunkCount := GetTotalChunks(client)
	if totalChunkCount < 20 {
		threshold = 2
	} else if totalChunkCount < 80 && totalChunkCount > 21 {
		threshold = 4
	}

	var minChunkCount float64 = -1
	var maxChunkCount float64 = 0
	shardChunkInfoAll := GetTotalChunksByShard(client)
	for _, shard := range *shardChunkInfoAll {
		if shard.Chunks > maxChunkCount {
			maxChunkCount = shard.Chunks
		}
		if minChunkCount == -1 || shard.Chunks < minChunkCount {
			minChunkCount = shard.Chunks
		}
	}

	// return true if the difference between the min and max is < the thresold
	chunkDifference := maxChunkCount - minChunkCount
	if chunkDifference < threshold {
		return 1
	}

	return 0
}

func (s *ShardingStats) Export(ch chan<- prometheus.Metric) {
	if s.Changelog != nil {
		s.Changelog.Export(ch)
	}
	if s.Topology != nil {
		s.Topology.Export(ch)
	}
	if s.Mongos != nil && s.BalancerLock != nil {
		mongosBalancerLockWho := strings.Split(s.BalancerLock.Who, ":")
		mongosBalancerLockHostPort := mongosBalancerLockWho[0] + ":" + mongosBalancerLockWho[1]
		mongosBalancerLockTimestamp.WithLabelValues(mongosBalancerLockHostPort).Set(float64(s.BalancerLock.When.Unix()))
		for _, mongos := range *s.Mongos {
			mongosUpSecs.WithLabelValues(mongos.Name).Set(mongos.Up)
			mongosPing.WithLabelValues(mongos.Name).Set(float64(mongos.Ping.Unix()))
			mongosBalancerLockState.WithLabelValues(mongos.Name).Set(-1)
			if mongos.Name == mongosBalancerLockHostPort {
				mongosBalancerLockState.WithLabelValues(mongos.Name).Set(s.BalancerLock.State)
			}
		}
	}
	balancerIsEnabled.Set(s.BalancerEnabled)
	balancerChunksBalanced.Set(s.IsBalanced)

	balancerIsEnabled.Collect(ch)
	balancerChunksBalanced.Collect(ch)
	mongosUpSecs.Collect(ch)
	mongosPing.Collect(ch)

	if shared.MongoServerVersionLessThan("3.6", s.Client) {
		mongosBalancerLockState.Collect(ch)
		mongosBalancerLockTimestamp.Collect(ch)
	}
}

func (s *ShardingStats) Describe(ch chan<- *prometheus.Desc) {
	if s.Changelog != nil {
		s.Changelog.Describe(ch)
	}
	if s.Topology != nil {
		s.Topology.Describe(ch)
	}
	balancerIsEnabled.Describe(ch)
	balancerChunksBalanced.Describe(ch)
	mongosUpSecs.Describe(ch)
	mongosPing.Describe(ch)

	if shared.MongoServerVersionLessThan("3.6", s.Client) {
		mongosBalancerLockState.Describe(ch)
		mongosBalancerLockTimestamp.Describe(ch)
	}
}

// GetShardingStatus gets sharding status.
func GetShardingStatus(client *mongo.Client) *ShardingStats {
	results := &ShardingStats{
		Client: client,
	}

	results.IsBalanced = IsClusterBalanced(client)
	results.BalancerEnabled = IsBalancerEnabled(client)
	results.Changelog = GetShardingChangelogStatus(client)
	results.Topology = GetShardingTopoStatus(client)
	results.Mongos = GetMongosInfo(client)

	if shared.MongoServerVersionLessThan("3.6", client) {
		results.BalancerLock = GetMongosBalancerLock(client)
	}

	return results
}
