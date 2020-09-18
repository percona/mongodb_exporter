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
	"fmt"
	"sort"
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	ti, err := newTopologyInfo(ctx, client)
	require.NoError(t, err)

	c := &diagnosticDataCollector{
		client:         client,
		logger:         logrus.New(),
		compatibleMode: true,
		topologyInfo:   ti,
	}

	metrics := helpers.CollectMetrics(c)
	actualMetrics := helpers.ReadMetrics(metrics)
	filters := []string{
		"mongodb_mongod_metrics_cursor_open",
		"mongodb_mongod_metrics_get_last_error_wtimeouts_total",
		"mongodb_mongod_wiredtiger_cache_bytes",
		"mongodb_mongod_wiredtiger_transactions_total",
		"mongodb_mongod_wiredtiger_cache_bytes_total",
		"mongodb_op_counters_total",
		"mongodb_ss_mem_resident",
		"mongodb_ss_mem_virtual",
		"mongodb_ss_metrics_cursor_open",
		"mongodb_ss_metrics_getLastError_wtime_totalMillis",
		"mongodb_ss_opcounters",
		"mongodb_ss_opcountersRepl",
		"mongodb_ss_wt_cache_maximum_bytes_configured",
		"mongodb_ss_wt_cache_modified_pages_evicted",
	}
	actualMetrics = filterMetrics(actualMetrics, filters)
	actualLines := helpers.Format(helpers.WriteMetrics(actualMetrics))
	metricNames := getMetricNames(actualLines)

	sort.Strings(filters)
	for _, want := range filters {
		assert.True(t, metricNames[want], fmt.Sprintf("missing %q metric", want))
	}
}
