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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestCollStatsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx) //nolint

	defer func() {
		err := database.Drop(ctx)
		assert.NoError(t, err)
	}()

	for i := 0; i < 3; i++ {
		coll := fmt.Sprintf("testcol_%02d", i)
		_, err := database.Collection(coll).InsertOne(ctx, bson.M{"f1": 1, "f2": "2"})
		assert.NoError(t, err)
	}

	ti := labelsGetterMock{}

	c := &collstatsCollector{
		client:       client,
		collections:  []string{"testdb.testcol_00", "testdb.testcol_01", "testdb.testcol_02"},
		logger:       logrus.New(),
		topologyInfo: ti,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_testdb_testcol_00_latencyStats_commands_latency testdb.testcol_00.latencyStats.commands.
# TYPE mongodb_testdb_testcol_00_latencyStats_commands_latency untyped
mongodb_testdb_testcol_00_latencyStats_commands_latency{collection="testcol_00",database="testdb"} 0
# HELP mongodb_testdb_testcol_01_latencyStats_commands_latency testdb.testcol_01.latencyStats.commands.
# TYPE mongodb_testdb_testcol_01_latencyStats_commands_latency untyped
mongodb_testdb_testcol_01_latencyStats_commands_latency{collection="testcol_01",database="testdb"} 0
# HELP mongodb_testdb_testcol_02_latencyStats_commands_latency testdb.testcol_02.latencyStats.commands.
# TYPE mongodb_testdb_testcol_02_latencyStats_commands_latency untyped
mongodb_testdb_testcol_02_latencyStats_commands_latency{collection="testcol_02",database="testdb"} 0` +
		"\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_testdb_testcol_00_latencyStats_commands_latency",
		"mongodb_testdb_testcol_01_latencyStats_commands_latency",
		"mongodb_testdb_testcol_02_latencyStats_commands_latency",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
