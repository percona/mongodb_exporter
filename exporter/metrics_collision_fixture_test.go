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

	systemMetrics, ok := loadFixture(t, diagnosticData83Fixture)["systemMetrics"].(bson.M)
	require.True(t, ok)

	labels := map[string]string{"cl_id": "", "cl_role": ""}
	metricsByName := gatherFixtureMetrics(t, makeMetrics("systemMetrics", systemMetrics, labels, false))

	collided, ok := metricsByName["mongodb_sys_ethtool_ens192_giant_hdr"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"0", "1"}, labelValues(collided, collisionLabel))

	// Counters with a unique name keep their plain identity.
	unique, ok := metricsByName["mongodb_sys_ethtool_ens192_ucast_pkts_tx"]
	require.True(t, ok)
	assert.Empty(t, labelValues(unique, collisionLabel))
}
