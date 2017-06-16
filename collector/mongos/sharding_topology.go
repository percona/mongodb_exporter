package collector_mongos

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	shardingTopoInfoTotalShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "shards_total",
		Help:      "Total # of Shards in the Cluster",
	})
	shardingTopoInfoDrainingShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "shards_draining_total",
		Help:      "Total # of Shards in the Cluster in draining state",
	})
	shardingTopoInfoTotalChunks = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "chunks_total",
		Help:      "Total # of Chunks in the Cluster",
	})
	shardingTopoInfoShardChunks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "shard_chunks_total",
		Help:      "Total number of chunks per shard",
	}, []string{"shard"})
	shardingTopoInfoTotalDatabases = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "databases_total",
		Help:      "Total # of Databases in the Cluster",
	}, []string{"type"})
	shardingTopoInfoTotalCollections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "collections_total",
		Help:      "Total # of Collections with Sharding enabled",
	})
)

type ShardingTopoShardInfo struct {
	Shard    string `bson:"_id"`
	Host     string `bson:"host"`
	Draining bool   `bson:"draining",omitifempty`
}

type ShardingTopoChunkInfo struct {
	Shard  string  `bson:"_id"`
	Chunks float64 `bson:"count"`
}

type ShardingTopoStatsTotalDatabases struct {
	Partitioned bool    `bson:"_id"`
	Total       float64 `bson:"total"`
}

type ShardingTopoStats struct {
	TotalChunks      float64
	TotalCollections float64
	TotalDatabases   *[]ShardingTopoStatsTotalDatabases
	Shards           *[]ShardingTopoShardInfo
	ShardChunks      *[]ShardingTopoChunkInfo
}

func GetShards(session *mgo.Session) *[]ShardingTopoShardInfo {
	var shards []ShardingTopoShardInfo
	err := session.DB("config").C("shards").Find(bson.M{}).All(&shards)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.shards'!")
	}
	return &shards
}

func GetTotalChunks(session *mgo.Session) float64 {
	chunkCount, err := session.DB("config").C("chunks").Find(bson.M{}).Count()
	if err != nil {
		glog.Error("Failed to execute find query on 'config.chunks'!")
	}
	return float64(chunkCount)
}

func GetTotalChunksByShard(session *mgo.Session) *[]ShardingTopoChunkInfo {
	var results []ShardingTopoChunkInfo
	err := session.DB("config").C("chunks").Pipe([]bson.M{{"$group": bson.M{"_id": "$shard", "count": bson.M{"$sum": 1}}}}).All(&results)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.chunks'!")
	}
	return &results
}

func GetTotalDatabases(session *mgo.Session) *[]ShardingTopoStatsTotalDatabases {
	results := []ShardingTopoStatsTotalDatabases{}
	query := []bson.M{{"$match": bson.M{"_id": bson.M{"$ne": "admin"}}}, {"$group": bson.M{"_id": "$partitioned", "total": bson.M{"$sum": 1}}}}
	err := session.DB("config").C("databases").Pipe(query).All(&results)
	if err != nil {
		glog.Error("Failed to execute find query on 'config.databases'!")
	}
	return &results
}

func GetTotalShardedCollections(session *mgo.Session) float64 {
	collCount, err := session.DB("config").C("collections").Find(bson.M{"dropped": false}).Count()
	if err != nil {
		glog.Error("Failed to execute find query on 'config.collections'!")
	}
	return float64(collCount)
}

func (status *ShardingTopoStats) Export(ch chan<- prometheus.Metric) {
	if status.Shards != nil {
		var drainingShards float64 = 0
		for _, shard := range *status.Shards {
			if shard.Draining == true {
				drainingShards = drainingShards + 1
			}
		}
		shardingTopoInfoDrainingShards.Set(drainingShards)
		shardingTopoInfoTotalShards.Set(float64(len(*status.Shards)))
	}
	shardingTopoInfoTotalChunks.Set(status.TotalChunks)
	shardingTopoInfoTotalCollections.Set(status.TotalCollections)

	shardingTopoInfoTotalDatabases.WithLabelValues("partitioned").Set(0)
	shardingTopoInfoTotalDatabases.WithLabelValues("unpartitioned").Set(0)
	if status.TotalDatabases != nil {
		for _, item := range *status.TotalDatabases {
			switch item.Partitioned {
			case true:
				shardingTopoInfoTotalDatabases.WithLabelValues("partitioned").Set(item.Total)
			case false:
				shardingTopoInfoTotalDatabases.WithLabelValues("unpartitioned").Set(item.Total)
			}
		}
	}

	if status.ShardChunks != nil {
		// set all known shards to zero first so that shards with zero chunks are still displayed properly
		for _, shard := range *status.Shards {
			shardingTopoInfoShardChunks.WithLabelValues(shard.Shard).Set(0)
		}
		for _, shard := range *status.ShardChunks {
			shardingTopoInfoShardChunks.WithLabelValues(shard.Shard).Set(shard.Chunks)
		}
	}

	shardingTopoInfoTotalShards.Collect(ch)
	shardingTopoInfoDrainingShards.Collect(ch)
	shardingTopoInfoTotalChunks.Collect(ch)
	shardingTopoInfoShardChunks.Collect(ch)
	shardingTopoInfoTotalCollections.Collect(ch)
	shardingTopoInfoTotalDatabases.Collect(ch)
}

func (status *ShardingTopoStats) Describe(ch chan<- *prometheus.Desc) {
	shardingTopoInfoTotalShards.Describe(ch)
	shardingTopoInfoDrainingShards.Describe(ch)
	shardingTopoInfoTotalChunks.Describe(ch)
	shardingTopoInfoShardChunks.Describe(ch)
	shardingTopoInfoTotalDatabases.Describe(ch)
	shardingTopoInfoTotalCollections.Describe(ch)
}

func GetShardingTopoStatus(session *mgo.Session) *ShardingTopoStats {
	results := &ShardingTopoStats{}

	results.Shards = GetShards(session)
	results.TotalChunks = GetTotalChunks(session)
	results.ShardChunks = GetTotalChunksByShard(session)
	results.TotalDatabases = GetTotalDatabases(session)
	results.TotalCollections = GetTotalShardedCollections(session)

	return results
}
