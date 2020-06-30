package exporter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestDiagnosticDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := getTestClient(ctx, t)

	c := &diagnosticDataCollector{
		ctx:    ctx,
		client: client,
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
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
