package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDescribeCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping integration test")
	}

	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "mongodb://localhost:27017"})

	ch := make(chan *prometheus.Desc)
	go collector.Describe(ch)
}

func TestCollectCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping integration test")
	}

	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "mongodb://localhost:27017"})

	ch := make(chan prometheus.Metric)
	go collector.Collect(ch)
}
