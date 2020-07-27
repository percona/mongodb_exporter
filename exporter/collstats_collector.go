package exporter

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type collstatsCollector struct {
	client      *mongo.Client
	collections []string
}

func (d *collstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *collstatsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.TODO()

	for _, dbCollection := range d.collections {
		parts := strings.Split(dbCollection, ".")
		if len(parts) != 2 { //nolint:gomnd
			continue
		}

		database := parts[0]
		collection := parts[1]

		aggregation := bson.D{
			{Key: "$collStats", Value: bson.M{"latencyStats": bson.E{Key: "histograms", Value: true}}},
		}

		cursor, err := d.client.Database(database).Collection(collection).Aggregate(ctx, mongo.Pipeline{aggregation})
		if err != nil {
			logrus.Errorf("cannot get $collstats cursor for collection %s.%s: %s", database, collection, err)
			continue
		}

		var stats []bson.M
		if err = cursor.All(ctx, &stats); err != nil {
			logrus.Errorf("cannot get $collstats for collection %s.%s: %s", database, collection, err)
			continue
		}

		for _, m := range stats {
			for _, metric := range buildMetrics(m) {
				ch <- metric
			}
		}
	}
}

var _ prometheus.Collector = (*collstatsCollector)(nil)
