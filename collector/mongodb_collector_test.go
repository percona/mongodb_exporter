package collector

import (
	"testing"

	"github.com/dcu/mongodb_exporter/shared"
	"github.com/prometheus/client_golang/prometheus"
)

func Test_CollectServerStatus(t *testing.T) {
	shared.ParseEnabledGroups("assers,durability,backgrond_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,memory,locks,metrics,cursors")
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})
	go collector.Collect(nil)
}

func Test_DescribeCollector(t *testing.T) {
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})

	ch := make(chan *prometheus.Desc)
	go collector.Describe(ch)
}

func Test_CollectCollector(t *testing.T) {
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})

	ch := make(chan prometheus.Metric)
	go collector.Collect(ch)
}
