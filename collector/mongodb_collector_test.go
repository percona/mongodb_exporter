package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/percona/mongodb_exporter/shared"
)

func TestCollectServerStatus(t *testing.T) {
	shared.ParseEnabledGroups("assers,durability,backgrond_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,memory,locks,metrics,cursors")
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})
	go collector.Collect(nil)
}

func TestDescribeCollector(t *testing.T) {
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})

	ch := make(chan *prometheus.Desc)
	go collector.Describe(ch)
}

func TestCollectCollector(t *testing.T) {
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})

	ch := make(chan prometheus.Metric)
	go collector.Collect(ch)
}
