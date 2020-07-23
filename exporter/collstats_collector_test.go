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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/Percona-Lab/mnogo_exporter/internal/tu"
)

func TestCollStatsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx) //nolint
	for i := 0; i < 3; i++ {
		col := fmt.Sprintf("c%d", i)
		for j := 0; j < 100; j++ {
			_, err := database.Collection(col).InsertOne(ctx, bson.M{"f1": 1, "f2": "2"})
			assert.NoError(t, err)
		}
	}

	c := &collstatsCollector{
		ctx:         ctx,
		client:      client,
		collections: []string{"testdb.c0", "testdb.c1", "testdb.c2"},
	}

	// It is important to keep repeated metric names from different collections because
	// there cannot be duplicated metric names so, testing for repeated collection names
	// we ensure that metric naming with prefixes and labels are working ok.
	expected := strings.NewReader(`
# HELP mongodb_testdb_c0_capped testdb.c0.
# TYPE mongodb_testdb_c0_capped untyped
mongodb_testdb_c0_capped{collection="c0",database="testdb"} 0
# HELP mongodb_testdb_c1_capped testdb.c1.
# TYPE mongodb_testdb_c1_capped untyped
mongodb_testdb_c1_capped{collection="c1",database="testdb"} 0
# HELP mongodb_testdb_c2_capped testdb.c2.
# TYPE mongodb_testdb_c2_capped untyped
mongodb_testdb_c2_capped{collection="c2",database="testdb"} 0` + "\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_testdb_c0_capped",
		"mongodb_testdb_c1_capped",
		"mongodb_testdb_c2_capped",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
