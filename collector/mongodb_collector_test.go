package collector

import (
	"github.com/dcu/mongodb_exporter/shared"
	"github.com/prometheus/client_golang/prometheus"
	"testing"
)

func Test_CollectServerStatus(t *testing.T) {
	shared.ParseEnabledGroups("assers,durability,backgrond_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,memory,locks,metrics,cursors")
	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "localhost"})
	serverStatus := collector.collectServerStatus(nil)

	if serverStatus.Asserts == nil {
		t.Error("Error loading document.")
	}
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

func Test_InvalidConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "s://localhost:123"})
	serverStatus := collector.collectServerStatus(nil)

	if serverStatus != nil {
		t.Fail()
	}
}
