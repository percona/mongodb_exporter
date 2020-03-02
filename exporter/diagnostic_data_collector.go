package exporter

import (
	"context"
	"fmt"

	// "github.com/go-kit/kit/log"
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
	cmd := bson.D{{"getDiagnosticData", "1"}}
	res := d.client.Database("admin").RunCommand(d.ctx, cmd)
	var m bson.M
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

	for _, metric := range d.makeMetrics("dd", m) {
		ch <- metric
	}
}

func (d *diagnosticDataCollector) makeMetrics(prefix string, m bson.M) []prometheus.Metric {
	var res []prometheus.Metric

	for k, v := range m {
		_ = k
		switch v := v.(type) {
		case bson.M:
			res = append(res, d.makeMetrics(prefix+"."+k, v)...)
		default:
			metric, err := makeRawMetric(prefix+"."+k, v)
			if err != nil {
				// TODO
				panic(err)
			}
			if metric != nil {
				res = append(res, metric)
			}
		}
	}

	return res
}

// check interface
var _ prometheus.Collector = (*diagnosticDataCollector)(nil)
