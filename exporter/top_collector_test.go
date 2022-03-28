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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestTopCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	ti := labelsGetterMock{}

	c := newTopCollector(ctx, client, logrus.New(), false, ti)

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_top_update_count",
	}
	filter = nil
	count := testutil.CollectAndCount(c, filter...)

	/*
		      The number of metrics is not a constant. It depends on the number of collections in the db.
			  It looks like:

		      # HELP mongodb_top_update_count top.update.
		      # TYPE mongodb_top_update_count untyped
		      mongodb_top_update_count{namespace="admin.system.roles"} 0
		      mongodb_top_update_count{namespace="admin.system.version"} 3
		      mongodb_top_update_count{namespace="config.cache.chunks.config.system.sessions"} 0
		      mongodb_top_update_count{namespace="config.cache.collections"} 1540
		      mongodb_top_update_count{namespace="config.image_collection"} 0
		      mongodb_top_update_count{namespace="config.system.sessions"} 12
		      mongodb_top_update_count{namespace="config.transaction_coordinators"} 0
		      mongodb_top_update_count{namespace="config.transactions"} 0
		      mongodb_top_update_count{namespace="local.oplog.rs"} 0
		      mongodb_top_update_count{namespace="local.replset.election"} 0
		      mongodb_top_update_count{namespace="local.replset.minvalid"} 0
		      mongodb_top_update_count{namespace="local.replset.oplogTruncateAfterPoint"} 0
		      mongodb_top_update_count{namespace="local.startup_log"} 0
		      mongodb_top_update_count{namespace="local.system.replset"} 0
		      mongodb_top_update_count{namespace="local.system.rollback.id"} 0

	*/
	assert.True(t, count > 0)
}
