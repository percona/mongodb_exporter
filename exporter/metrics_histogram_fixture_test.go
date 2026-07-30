// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// A MongoDB 8.3 reply carries both bucket shapes: the "histogram" arrays of
// serverStatus.opLatencies and the "histograms" nodes under serverStatus.metrics.query.
func TestServerStatusHistogramsFromFixture(t *testing.T) {
	t.Parallel()

	serverStatus, ok := loadDiagnosticData83Fixture(t)["serverStatus"].(bson.M)
	require.True(t, ok)

	labels := map[string]string{"cl_id": "", "cl_role": ""}
	metricsByName := gatherMetrics(t, makeMetricsWithHistograms("serverStatus", serverStatus, labels, false, true))

	// Every op type keeps its own bucket family.
	for _, opType := range []string{"reads", "writes", "commands", "transactions"} {
		assert.Contains(t, metricsByName, "mongodb_ss_opLatencies_"+opType+"_histogram_count", opType)
	}

	// Each bound keeps the count it was captured with, so a bound paired with the wrong count
	// fails here rather than showing up as a plausible looking heatmap.
	opLatencyBuckets, ok := metricsByName["mongodb_ss_opLatencies_reads_histogram_count"]
	require.True(t, ok)
	assert.Equal(t, map[string]float64{
		"0": 0, "8": 0, "64": 16737, "512": 947, "3072": 351,
		"8192": 77, "24576": 54, "65536": 13, "131072": 21,
	}, counterValuesByLabel(opLatencyBuckets, "lower_bound"))
	assert.NotContains(t, metricsByName, "mongodb_ss_opLatencies_reads_histogram_micros")

	plannerBuckets, ok := metricsByName["mongodb_ss_metrics_query_multiPlanner_histograms_classicWorks_count"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"0", "128", "256", "512", "1024", "2048", "4096", "8192", "16384", "32768"},
		labelValues(plannerBuckets, "lower_bound"))
}

func TestServerStatusHistogramsFromFixtureAreSkippedByDefault(t *testing.T) {
	t.Parallel()

	serverStatus, ok := loadDiagnosticData83Fixture(t)["serverStatus"].(bson.M)
	require.True(t, ok)

	labels := map[string]string{"cl_id": "", "cl_role": ""}
	metricsByName := gatherMetrics(t, makeMetrics("serverStatus", serverStatus, labels, false))

	for name := range metricsByName {
		assert.NotContains(t, name, "histogram")
	}

	// Fields next to the buckets are still collected.
	assert.Contains(t, metricsByName, "mongodb_ss_opLatencies_latency")
}
