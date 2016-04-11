package collector_mongos

import (
	"time"
	"strings"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	balancerIsEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:	Namespace,
			Subsystem:	"sharding",
			Name:		"balancer_enabled",
			Help:		"Boolean reporting if cluster balancer is enabled (1 = enabled/0 = disabled)",
	})
	balancerChunksBalanced = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:	Namespace,
			Subsystem:	"sharding",
			Name:		"chunks_is_balanced",
			Help:		"Boolean reporting if cluster chunks are evenly balanced across shards (1 = yes/0 = no)",
	})
	mongosClusterIdHex = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"cluster_id",
			Help:		"The hex representation of the Cluster ID",
	}, []string{"hex"})
	mongosClusterCurrentVersion = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"cluster_current_version",
			Help:		"The current metadata version number of the Cluster",
	})
	mongosClusterMinCompatVersion = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"cluster_min_compatible_version",
			Help:		"The minimum compatible metadata version number of the Cluster",
	})
	mongosUpSecs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"mongos_uptime_seconds",
			Help:		"The uptime of the Mongos processes in seconds",
	}, []string{"name","version"})
	mongosPing = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"mongos_last_ping_timestamp",
			Help:		"The unix timestamp of the last Mongos ping to the Cluster config servers",
	}, []string{"name","version"})
	mongosBalancerLockTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"balancer_lock_timestamp",
			Help:		"The unix timestamp of the last update to the Cluster balancer lock",
	})
	mongosBalancerLockState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:      Namespace,
			Subsystem:      "sharding",
			Name:		"balancer_lock_state",
			Help:		"The state of the Cluster balancer lock (-1 = none/0 = unlocked/1 = contention/2 = locked)",
	}, []string{"name","version"})
)

type ShardingVersion struct {
	MinCompatVersion	float64		`bson:"minCompatibleVersion"`
	CurrentVersion		float64		`bson:"currentVersion"`
	ClusterId		bson.ObjectId	`bson:"clusterId"`
}

type MongosInfo struct {
	Name		string		`bson:"_id"`
	Ping		time.Time       `bson:"ping"`
	Up		float64		`bson:"up"`
	Waiting		bool		`bson:"waiting"`
	MongoVersion    string		`bson:"mongoVersion"`
}

type MongosBalancerLock struct {
	State	float64		`bson:"state"`
	Process	string		`bson:"process"`
	Who	string		`bson:"who"`
	When	time.Time	`bson:"when"`
	Why	string		`bson:"why"`
}

type ShardingStats struct {
	IsBalanced	float64	
	BalancerEnabled	float64
	Changelog	*ShardingChangelogStats	
	Topology	*ShardingTopoStats
	BalancerLock	*MongosBalancerLock
	Version		*ShardingVersion
	Mongos		*[]MongosInfo
}

func GetShardingVersion(session *mgo.Session) *ShardingVersion {
	mongosVersion := &ShardingVersion{}
	err := session.DB("config").C("version").Find(bson.M{ "_id" : 1 }).One(&mongosVersion)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.version'!")
	}
	return mongosVersion
}

func GetMongosInfo(session *mgo.Session) *[]MongosInfo {
	mongosInfo := []MongosInfo{}
	err := session.DB("config").C("mongos").Find(bson.M{ "ping" : bson.M{ "$gte" : time.Now().Add(-10 * time.Minute) } }).All(&mongosInfo)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.mongos'!")
	}
	return &mongosInfo
}

func GetMongosBalancerLock(session *mgo.Session) *MongosBalancerLock {
	var balancerLock *MongosBalancerLock
	err := session.DB("config").C("locks").Find(bson.M{ "_id" : "balancer" }).One(&balancerLock)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.locks'!")
	}
	return balancerLock
}

func IsBalancerEnabled(session *mgo.Session) float64 {
	balancerConfig := struct {
		Stopped	bool	`bson:"stopped"`
	}{}
	err := session.DB("config").C("settings").Find(bson.M{ "_id" : "balancer" }).One(&balancerConfig)
	if err != nil {
		return 1
	}
	if balancerConfig.Stopped == true {
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
	if status.Version != nil {
		clusterId := status.Version.ClusterId.Hex()
		mongosClusterIdHex.WithLabelValues(clusterId).Set(1)
		mongosClusterCurrentVersion.Set(status.Version.CurrentVersion)
		mongosClusterMinCompatVersion.Set(status.Version.MinCompatVersion)
	}
	if status.Mongos != nil && status.BalancerLock != nil {
		mongosBalancerLockTimestamp.Set(float64(status.BalancerLock.When.Unix()))
		mongosBalancerLockWho := strings.Split(status.BalancerLock.Who, ":")
		mongosBalancerLockHostPort := mongosBalancerLockWho[0] + ":" + mongosBalancerLockWho[1]
		for _, mongos := range *status.Mongos {
			labels := prometheus.Labels{"name": mongos.Name, "version": mongos.MongoVersion }
			mongosUpSecs.With(labels).Set(mongos.Up)
			mongosPing.With(labels).Set(float64(mongos.Ping.Unix()))
			mongosBalancerLockState.With(labels).Set(-1)
			if mongos.Name == mongosBalancerLockHostPort {
				mongosBalancerLockState.With(labels).Set(status.BalancerLock.State)
			}
		}
	}
	balancerIsEnabled.Set(status.IsBalanced)
	balancerChunksBalanced.Set(status.BalancerEnabled)

	balancerIsEnabled.Collect(ch)
	balancerChunksBalanced.Collect(ch)
	mongosClusterIdHex.Collect(ch)
	mongosClusterCurrentVersion.Collect(ch)
	mongosClusterMinCompatVersion.Collect(ch)
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
	mongosClusterIdHex.Describe(ch)
	mongosClusterCurrentVersion.Describe(ch)
	mongosClusterMinCompatVersion.Describe(ch)
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
	results.Version = GetShardingVersion(session)
	results.Mongos = GetMongosInfo(session)
	results.BalancerLock = GetMongosBalancerLock(session)

	return results
}
