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
	//filter := []string{
	//	"mongodb_top_update_count",
	//}
	var filter []string
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
