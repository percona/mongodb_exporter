// mongodb_exporter
// Copyright (C) 2026 Percona LLC
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
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestReplSetMetricsReplicationLag(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		primary     int64
		secondary   int64
		omitPrimary bool
		wantLag     string
	}{
		{name: "secondary newer than primary heartbeat", primary: 1000, secondary: 1010, wantLag: "0"},
		{name: "secondary caught up", primary: 1000, secondary: 1000, wantLag: "0"},
		{name: "secondary behind primary", primary: 1017, secondary: 1000, wantLag: "17"},
		{name: "delayed secondary", primary: 1060, secondary: 1000, wantLag: "60"},
		{name: "no primary", secondary: 1000, omitPrimary: true},
		{name: "primary optime unavailable", secondary: 1000},
		{name: "secondary optime unavailable", primary: 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			members := bson.A{bson.M{
				"name":       "secondary:27017",
				"stateStr":   "SECONDARY",
				"health":     1,
				"self":       true,
				"optimeDate": primitive.DateTime(tc.secondary * 1000),
			}}
			if !tc.omitPrimary {
				members = append(members, bson.M{
					"name":       "primary:27017",
					"stateStr":   "PRIMARY",
					"health":     1,
					"optimeDate": primitive.DateTime(tc.primary * 1000),
				})
			}

			metrics := replSetMetrics(bson.M{"set": "rs0", "members": members}, promslog.New(&promslog.Config{}))
			var expected string
			if tc.wantLag != "" {
				expected = fmt.Sprintf(`
# HELP mongodb_mongod_replset_member_replication_lag The replication lag that this member has with the primary.
# TYPE mongodb_mongod_replset_member_replication_lag gauge
mongodb_mongod_replset_member_replication_lag{name="secondary:27017",self="1",set="rs0",state="SECONDARY"} %s
`, tc.wantLag)
			}

			err := testutil.CollectAndCompare(staticCollector(metrics), strings.NewReader(expected), "mongodb_mongod_replset_member_replication_lag")
			require.NoError(t, err)
		})
	}
}
