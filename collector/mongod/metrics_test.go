package mongod

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestReplStatsExportShouldNotPanic(t *testing.T) {
	rs := &ReplStats{}
	ch := make(chan prometheus.Metric)
	f := func() { rs.Export(ch) }

	assert.NotPanics(t, f, "nil pointer in ReplStats")
}

func TestPreloadStatsExportShouldNotPanic(t *testing.T) {
	ps := &PreloadStats{}
	ch := make(chan prometheus.Metric)
	f := func() { ps.Export(ch) }

	assert.NotPanics(t, f, "nil pointer in PreloadStats")
}
