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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestDiagnosticDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	ti := labelsGetterMock{}

	c := &diagnosticDataCollector{
		client:       client,
		logger:       logrus.New(),
		topologyInfo: ti,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_oplog_stats_ok local.oplog.rs.stats.
# TYPE mongodb_oplog_stats_ok untyped
mongodb_oplog_stats_ok{assert_type="msg",cl_id="5f4da51a76bfb5fe22797fcf",cl_role="shardsvr",cmd_name="<UNKNOWN>",concern_type="local",conn_type="active",count_type="writers",csr_type="noTimeout",doc_op_type="updated",legacy_op_type="command",perf_bucket="operation write latency histogram (bucket 3) - 500-999us",rs_nm="rs1",rs_state="1"} 1
# HELP mongodb_oplog_stats_wt_btree_fixed_record_size local.oplog.rs.stats.wiredTiger.btree.
# TYPE mongodb_oplog_stats_wt_btree_fixed_record_size untyped
mongodb_oplog_stats_wt_btree_fixed_record_size{assert_type="msg",cl_id="5f4da51a76bfb5fe22797fcf",cl_role="shardsvr",cmd_name="<UNKNOWN>",concern_type="local",conn_type="active",count_type="writers",csr_type="noTimeout",doc_op_type="updated",legacy_op_type="command",perf_bucket="operation write latency histogram (bucket 3) - 500-999us",rs_nm="rs1",rs_state="1"} 0` +
		"\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	//filter := []string{
	//	"mongodb_oplog_stats_ok",
	//	"mongodb_oplog_stats_wt_btree_fixed_record_size",
	//	//"mongodb_oplog_stats_wt_transaction_update_conflicts",
	//}
	// err := testutil.CollectAndCompare(c, expected, filter...)
	err := testutil.CollectAndCompare(c, expected)
	assert.NoError(t, err)
}
