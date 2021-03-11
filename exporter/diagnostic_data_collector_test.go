// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package exporter

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestDiagnosticDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	logger := logrus.New()
	ti := labelsGetterMock{}

	c := &diagnosticDataCollector{
		client:       client,
		logger:       logger,
		topologyInfo: ti,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_oplog_stats_ok local.oplog.rs.stats.
# TYPE mongodb_oplog_stats_ok untyped
mongodb_oplog_stats_ok 1
# HELP mongodb_oplog_stats_wt_btree_fixed_record_size local.oplog.rs.stats.wiredTiger.btree.
# TYPE mongodb_oplog_stats_wt_btree_fixed_record_size untyped
mongodb_oplog_stats_wt_btree_fixed_record_size 0
# HELP mongodb_oplog_stats_wt_transaction_update_conflicts local.oplog.rs.stats.wiredTiger.transaction.
# TYPE mongodb_oplog_stats_wt_transaction_update_conflicts untyped
mongodb_oplog_stats_wt_transaction_update_conflicts 0` + "\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_oplog_stats_ok",
		"mongodb_oplog_stats_wt_btree_fixed_record_size",
		"mongodb_oplog_stats_wt_transaction_update_conflicts",
	}
	// TODO: use NewPedanticRegistry when mongodb_exporter code fulfils its requirements (https://jira.percona.com/browse/PMM-6630).
	reg := prometheus.NewRegistry()
	err := reg.Register(c)
	require.NoError(t, err)
	err = testutil.GatherAndCompare(reg, expected, filter...)
	assert.NoError(t, err)
}

func TestAllDiagnosticDataCollectorMetrics(t *testing.T) {
	// if inGithubActions() {
	// 	t.Skip("Test not reliable in Gihub Actions")
	// }

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	ti := labelsGetterMock{}
	log := logrus.New()

	c := &diagnosticDataCollector{
		client:         client,
		logger:         log,
		compatibleMode: true,
		topologyInfo:   ti,
	}

	metrics := helpers.CollectMetrics(c)

	/*
	  How to regenerate this file:
	  1. Delete it
	  2. Start the sandbox with the highest available version
	  3. Run tests updating samples: UPDATE_SAMPLES=1 make test
	  4. Stop the sandbox and restart it with a lower MongoDB version.
	  5. Repeat 3 & 4 for all MongoDB versions.

	  First run will save ALL available metrics in newer MongoDB versions
	  and subsequent runs will remove the metrics not available in previous
	  versions. At the end you will have the base list of metrics, common
	  to all MongoDB versions
	*/
	compareMetrics(t, metrics, "testdata/diagnostic_data_base.json")
}

func compareMetrics(t *testing.T, metrics []prometheus.Metric, wantFile string) {
	actualMetrics := helpers.ReadMetrics(metrics)
	actualLines := helpers.Format(helpers.WriteMetrics(actualMetrics))

	metricNames := getMetricNames(actualLines)

	updateSamples, _ := strconv.ParseBool(os.Getenv("UPDATE_SAMPLES"))

	var wantNames map[string]bool
	// ignore error because when we are regenerating the file, it might not exist
	readJSON(wantFile, &wantNames) //nolint:errcheck

	// don't use assert.Equal because since metrics are dynamic, we don't always have the same
	// metric names in all environments so, we should only compare against a list of commonly
	// available metrics.
	for name := range wantNames {
		_, ok := metricNames[name]
		assert.True(t, ok, name+" metric is missing")
		if !ok {
			delete(wantNames, name)
		}
	}

	if updateSamples {
		if len(wantNames) > 0 {
			assert.NoError(t, writeJSON(wantFile, wantNames))
		} else {
			// if we are regenerating the data we need to write the data we got from MongoDB
			assert.NoError(t, writeJSON(wantFile, metricNames))
		}
	}
}
