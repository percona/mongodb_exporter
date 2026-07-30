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

// The vmxnet3 NIC of the reported host exposes two "giant hdr" counters that differ only
// in leading whitespace, which used to make the whole systemMetrics tree unexportable.
func TestSystemMetricsCollisionsFromFixture(t *testing.T) {
	t.Parallel()

	systemMetrics, ok := loadDiagnosticData83Fixture(t)["systemMetrics"].(bson.M)
	require.True(t, ok)

	labels := map[string]string{"cl_id": "", "cl_role": ""}
	metricsByName := gatherFixtureMetrics(t, makeMetrics("systemMetrics", systemMetrics, labels, false))

	collided, ok := metricsByName["mongodb_sys_ethtool_ens192_giant_hdr"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"     giant hdr", "  giant hdr"},
		labelValues(collided, collisionLabel))

	// Counters with a unique name keep their plain identity.
	unique, ok := metricsByName["mongodb_sys_ethtool_ens192_ucast_pkts_tx"]
	require.True(t, ok)
	assert.Empty(t, labelValues(unique, collisionLabel))
}

// The whole captured reply has to survive a scrape, not only the ethtool subtree, and it is the
// only place where the 8.3 payload is walked with histograms enabled.
func TestDiagnosticData83Gathers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name              string
		includeHistograms bool
	}{
		{name: "without histograms"},
		{name: "with histograms", includeHistograms: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			labels := map[string]string{"cl_id": "", "cl_role": ""}
			metricsByName := gatherFixtureMetrics(t, makeMetricsWithHistograms("",
				loadDiagnosticData83Fixture(t), labels, false, tc.includeHistograms))

			assert.Contains(t, metricsByName, "mongodb_sys_ethtool_ens192_giant_hdr")

			_, ok := metricsByName["mongodb_ss_metrics_query_cbr_histograms_micros_count"]
			assert.Equal(t, tc.includeHistograms, ok, "histogram buckets follow the flag")
		})
	}
}
