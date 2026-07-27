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

const diagnosticData83Fixture = "get_diagnostic_data_8.3.json"

// A MongoDB 8.3 reply carries both bucket shapes: the "histogram" arrays of
// serverStatus.opLatencies and the "histograms" nodes under serverStatus.metrics.query.
func TestServerStatusHistogramsFromFixture(t *testing.T) {
	t.Parallel()

	serverStatus, ok := loadFixture(t, diagnosticData83Fixture)["serverStatus"].(bson.M)
	require.True(t, ok)

	labels := map[string]string{"cl_id": "", "cl_role": ""}
	metricsByName := gatherFixtureMetrics(t, makeMetricsWithHistograms("serverStatus", serverStatus, labels, false, true))

	opLatencyBuckets, ok := metricsByName["mongodb_ss_opLatencies_reads_histogram_count"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"0", "8", "64", "512", "3072", "8192", "24576", "65536", "131072"},
		labelValues(opLatencyBuckets, "micros"))
	assert.NotContains(t, metricsByName, "mongodb_ss_opLatencies_reads_histogram_micros")

	plannerBuckets, ok := metricsByName["mongodb_ss_metrics_query_multiPlanner_histograms_classicWorks_count"]
	require.True(t, ok)
	assert.Len(t, labelValues(plannerBuckets, "lower_bound"), 10)
}

func TestServerStatusHistogramsFromFixtureAreSkippedByDefault(t *testing.T) {
	t.Parallel()

	serverStatus, ok := loadFixture(t, diagnosticData83Fixture)["serverStatus"].(bson.M)
	require.True(t, ok)

	labels := map[string]string{"cl_id": "", "cl_role": ""}
	metricsByName := gatherFixtureMetrics(t, makeMetrics("serverStatus", serverStatus, labels, false))

	for name := range metricsByName {
		assert.NotContains(t, name, "histogram")
	}

	// Fields next to the buckets are still collected.
	assert.Contains(t, metricsByName, "mongodb_ss_opLatencies_latency")
}
