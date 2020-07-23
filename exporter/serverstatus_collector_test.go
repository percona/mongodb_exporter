// mnogo_exporter
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
	"github.com/stretchr/testify/assert"

	"github.com/Percona-Lab/mnogo_exporter/internal/tu"
)

func TestServerStatusDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	c := &serverStatusCollector{
		ctx:    ctx,
		client: client,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_mem_bits mem.
# TYPE mongodb_mem_bits untyped
mongodb_mem_bits 64
# HELP mongodb_metrics_commands_cloneCollection_failed metrics.commands.cloneCollection.
# TYPE mongodb_metrics_commands_cloneCollection_failed untyped
mongodb_metrics_commands_cloneCollection_failed 0
# HELP mongodb_metrics_commands_connPoolSync_failed metrics.commands.connPoolSync.
# TYPE mongodb_metrics_commands_connPoolSync_failed untyped
mongodb_metrics_commands_connPoolSync_failed 0
# HELP mongodb_wiredTiger_log_slot_join_calls_yielded wiredTiger.log.
# TYPE mongodb_wiredTiger_log_slot_join_calls_yielded untyped
mongodb_wiredTiger_log_slot_join_calls_yielded 0` + "\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_mem_bits",
		"mongodb_metrics_commands_cloneCollection_failed",
		"mongodb_metrics_commands_connPoolSync_failed",
		"mongodb_wiredTiger_log_slot_join_calls_yielded",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
