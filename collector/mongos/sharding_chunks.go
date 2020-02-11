
package mongos

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

)

var (
	mongosShardChunks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "sharding",
		Name:      "namespace_shard_chunks",
		Help:      "The count of chunks per shard per collection",
	}, []string{"namespace","shard"})
)

type AggegateId struct {
	Namespace string `bson:"ns"`
	Shard     string `bson:"shard"`
}

type AggregateOut struct {
	Inner AggegateId `bson:"_id"`
	Count int64      `bson:"count"`
}

type ChunksInfo struct {
	Namespace string
	Shard     string
	Chunks    int64
}

type ShardingCounts struct {
	ShardChunks     *[]ChunksInfo
}

// GetShardChunks counts up chunks per collection per shard
func GetShardChunks(client *mongo.Client) *[]ChunksInfo {
	chunksInfo := []ChunksInfo{}

	matching := bson.M{}
	grouping := bson.M{"_id": bson.M{
		"ns":    "$ns",
		"shard": "$shard",
	},
		"count": bson.M{"$sum": 1},
	}

	c, err := client.Database("config").Collection("chunks").Aggregate(context.TODO(), []bson.M{{"$match": matching}, {"$group": grouping}})
	if err != nil {
		log.Errorf("Failed to execute aggregate on 'config.chunks': %s.", err)
		return nil
	}
	defer c.Close(context.TODO())

	for c.Next(context.TODO()) {
		i := &ChunksInfo{}
		a := &AggregateOut{}
		if err := c.Decode(a); err != nil {
			log.Error(err)
			continue
		}
		i.Namespace = a.Inner.Namespace
		i.Shard = a.Inner.Shard
		i.Chunks = a.Count
		chunksInfo = append(chunksInfo, *i)
	}

	return &chunksInfo
}

func (status *ShardingCounts) Export(ch chan<- prometheus.Metric) {
	if status.ShardChunks != nil {
		for _, chunkInfo := range *status.ShardChunks {
			// ...WithLabelValues().Set() only accepts float64 even though an
			// integer type is a better fit
			mongosShardChunks.WithLabelValues(chunkInfo.Namespace,chunkInfo.Shard).Set(float64(chunkInfo.Chunks))
		}
	}
	mongosShardChunks.Collect(ch)
}

func (status *ShardingCounts) Describe(ch chan<- *prometheus.Desc) {
	mongosShardChunks.Describe(ch)
}

func GetChunkCountList(client *mongo.Client) *ShardingCounts {
	results := &ShardingCounts{}
	results.ShardChunks = GetShardChunks(client)
	return results
}
