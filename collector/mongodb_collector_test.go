package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping integration test")
	}

	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "mongodb://localhost:27017"})

	descCh := make(chan *prometheus.Desc)
	go func() {
		collector.Describe(descCh)
		close(descCh)
	}()
	metricCh := make(chan prometheus.Metric)
	go func() {
		collector.Collect(metricCh)
		close(metricCh)
	}()

	var descs, metrics int
	for range descCh {
		descs++
	}
	for range metricCh {
		metrics++
	}

	if descs != metrics {
		t.Errorf("got %d descs and %d metrics", descs, metrics)
	}
}
