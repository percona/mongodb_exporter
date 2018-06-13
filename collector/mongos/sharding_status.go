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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

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
}

func GetMongosInfo(session *mgo.Session) *[]MongosInfo {
	mongosInfo := []MongosInfo{}
	q := session.DB("config").C("mongos").Find(bson.M{"ping": bson.M{"$gte": time.Now().Add(-10 * time.Minute)}})
	if err := shared.AddCodeCommentToQuery(q).All(&mongosInfo); err != nil {
		log.Errorf("Failed to execute find query on 'config.mongos': %s.", err)
	}
	return &mongosInfo
}

func GetMongosBalancerLock(session *mgo.Session) *MongosBalancerLock {
	var balancerLock *MongosBalancerLock
	q := session.DB("config").C("locks").Find(bson.M{"_id": "balancer"})
	if err := shared.AddCodeCommentToQuery(q).One(&balancerLock); err != nil {
		log.Errorf("Failed to execute find query on 'config.locks': %s.", err)
	}
	return balancerLock
}

func IsBalancerEnabled(session *mgo.Session) float64 {
	balancerConfig := struct {
		Stopped bool `bson:"stopped"`
	}{}
	q := session.DB("config").C("settings").Find(bson.M{"_id": "balancer"})
	if err := shared.AddCodeCommentToQuery(q).One(&balancerConfig); err != nil {
		return 1
	}
	if balancerConfig.Stopped {
		return 0
	}
	return 1
}

func IsClusterBalanced(session *mgo.Session) float64 {
	// Different thresholds based on size
	// http://docs.mongodb.org/manual/core/sharding-internals/#sharding-migration-thresholds
	var threshold float64 = 8
	totalChunkCount := GetTotalChunks(session)
	if totalChunkCount < 20 {
		threshold = 2
	} else if totalChunkCount < 80 && totalChunkCount > 21 {
		threshold = 4
	}

	var minChunkCount float64 = -1
	var maxChunkCount float64 = 0
	shardChunkInfoAll := GetTotalChunksByShard(session)
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

func (status *ShardingStats) Export(ch chan<- prometheus.Metric) {
	if status.Changelog != nil {
		status.Changelog.Export(ch)
	}
	if status.Topology != nil {
		status.Topology.Export(ch)
	}
	if status.Mongos != nil && status.BalancerLock != nil {
		mongosBalancerLockWho := strings.Split(status.BalancerLock.Who, ":")
		mongosBalancerLockHostPort := mongosBalancerLockWho[0] + ":" + mongosBalancerLockWho[1]
		mongosBalancerLockTimestamp.WithLabelValues(mongosBalancerLockHostPort).Set(float64(status.BalancerLock.When.Unix()))
		for _, mongos := range *status.Mongos {
			mongosUpSecs.WithLabelValues(mongos.Name).Set(mongos.Up)
			mongosPing.WithLabelValues(mongos.Name).Set(float64(mongos.Ping.Unix()))
			mongosBalancerLockState.WithLabelValues(mongos.Name).Set(-1)
			if mongos.Name == mongosBalancerLockHostPort {
				mongosBalancerLockState.WithLabelValues(mongos.Name).Set(status.BalancerLock.State)
			}
		}
	}
	balancerIsEnabled.Set(status.BalancerEnabled)
	balancerChunksBalanced.Set(status.IsBalanced)

	balancerIsEnabled.Collect(ch)
	balancerChunksBalanced.Collect(ch)
	mongosUpSecs.Collect(ch)
	mongosPing.Collect(ch)
	mongosBalancerLockState.Collect(ch)
	mongosBalancerLockTimestamp.Collect(ch)
}

func (status *ShardingStats) Describe(ch chan<- *prometheus.Desc) {
	if status.Changelog != nil {
		status.Changelog.Describe(ch)
	}
	if status.Topology != nil {
		status.Topology.Describe(ch)
	}
	balancerIsEnabled.Describe(ch)
	balancerChunksBalanced.Describe(ch)
	mongosUpSecs.Describe(ch)
	mongosPing.Describe(ch)
	mongosBalancerLockState.Describe(ch)
	mongosBalancerLockTimestamp.Describe(ch)
}

func GetShardingStatus(session *mgo.Session) *ShardingStats {
	results := &ShardingStats{}

	results.IsBalanced = IsClusterBalanced(session)
	results.BalancerEnabled = IsBalancerEnabled(session)
	results.Changelog = GetShardingChangelogStatus(session)
	results.Topology = GetShardingTopoStatus(session)
	results.Mongos = GetMongosInfo(session)
	results.BalancerLock = GetMongosBalancerLock(session)

	return results
}
