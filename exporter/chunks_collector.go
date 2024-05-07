package exporter

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

type chunksCollector struct {
	ctx        context.Context
	base       *baseCollector
	compatible bool
}

// newChunksCollector creates collector collecting metrics about chunks for shards Mongo.
func newChunksCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, compatibleMode bool) *chunksCollector {
	return &chunksCollector{
		ctx:        ctx,
		base:       newBaseCollector(client, logger.WithFields(logrus.Fields{"collector": "shards"})),
		compatible: compatibleMode,
	}
}

func (d *chunksCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *chunksCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *chunksCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "chunks")()

	client := d.base.client
	l := d.base.logger
	ctx := d.ctx

	metrics := make([]prometheus.Metric, 0)

	l.Debugf("chunksTotal")
	metric, err := chunksTotal(ctx, client)
	if err != nil {
		l.Debugf("cannot create metric for chunks total: %s", err)
	} else {
		metrics = append(metrics, metric)
	}

	l.Debugf("chunksTotalPerShard")
	ms, err := chunksTotalPerShard(ctx, client)
	if err != nil {
		l.Debugf("cannot create metric for chunks total per shard: %s", err)
	} else {
		metrics = append(metrics, ms...)
	}

	for _, metric := range metrics {
		ch <- metric
	}
}

func chunksTotal(ctx context.Context, client *mongo.Client) (prometheus.Metric, error) {
	n, err := client.Database("config").Collection("chunks").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	name := "mongodb_mongos_sharding_chunks_total"
	help := "Total number of chunks"

	d := prometheus.NewDesc(name, help, nil, nil)
	return prometheus.NewConstMetric(d, prometheus.GaugeValue, float64(n))
}

func chunksTotalPerShard(ctx context.Context, client *mongo.Client) ([]prometheus.Metric, error) {
	aggregation := bson.D{
		{Key: "$group", Value: bson.M{"_id": "$shard", "count": bson.M{"$sum": 1}}},
	}

	cursor, err := client.Database("config").Collection("chunks").Aggregate(ctx, mongo.Pipeline{aggregation})
	if err != nil {
		return nil, err
	}

	var shards []bson.M
	if err = cursor.All(ctx, &shards); err != nil {
		return nil, err
	}

	metrics := make([]prometheus.Metric, 0, len(shards))

	for _, shard := range shards {
		help := "Total number of chunks per shard"
		labels := map[string]string{"shard": shard["_id"].(string)}

		d := prometheus.NewDesc("mongodb_mongos_sharding_shard_chunks_total", help, nil, labels)
		val, ok := shard["count"].(int32)
		if !ok {
			continue
		}

		metric, err := prometheus.NewConstMetric(d, prometheus.GaugeValue, float64(val))
		if err != nil {
			continue
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}
