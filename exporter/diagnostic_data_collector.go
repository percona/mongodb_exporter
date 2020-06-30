package exporter

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type diagnosticDataCollector struct {
	ctx    context.Context
	client *mongo.Client
	// l      log.Logger
}

func (d *diagnosticDataCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *diagnosticDataCollector) Collect(ch chan<- prometheus.Metric) {
	var m bson.M

	cmd := bson.D{{Key: "getDiagnosticData", Value: "1"}}
	res := d.client.Database("admin").RunCommand(d.ctx, cmd)

	if err := res.Decode(&m); err != nil {
		ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
		return
	}

	m, ok := m["data"].(bson.M)
	if !ok {
		err := fmt.Errorf("unexpected %T for data", m["data"])
		ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)

		return
	}

	for _, metric := range makeMetrics("", m, nil) {
		ch <- metric
	}
}

// check interface.
var _ prometheus.Collector = (*diagnosticDataCollector)(nil)
