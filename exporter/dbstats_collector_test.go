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

func TestDBStatsCollector(t *testing.T) {
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
		for j := 0; j < 10; j++ {
			_, err := database.Collection(coll).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
			assert.NoError(t, err)
		}
	}

	ti := labelsGetterMock{}

	c := &dbstatsCollector{
		client:       client,
		logger:       logrus.New(),
		topologyInfo: ti,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_dbstats_testdb_avgObjSize dbstats_testdb.
# TYPE mongodb_dbstats_testdb_avgObjSize untyped
mongodb_dbstats_testdb_avgObjSize 40
# HELP mongodb_dbstats_testdb_collections dbstats_testdb.
# TYPE mongodb_dbstats_testdb_collections untyped
mongodb_dbstats_testdb_collections 3
# HELP mongodb_dbstats_testdb_dataSize dbstats_testdb.
# TYPE mongodb_dbstats_testdb_dataSize untyped
mongodb_dbstats_testdb_dataSize 1200
# HELP mongodb_dbstats_testdb_indexSize dbstats_testdb.
# TYPE mongodb_dbstats_testdb_indexSize untyped
mongodb_dbstats_testdb_indexSize 12288
# HELP mongodb_dbstats_testdb_indexes dbstats_testdb.
# TYPE mongodb_dbstats_testdb_indexes untyped
mongodb_dbstats_testdb_indexes 3
# HELP mongodb_dbstats_testdb_objects dbstats_testdb.
# TYPE mongodb_dbstats_testdb_objects untyped
mongodb_dbstats_testdb_objects 30
# HELP mongodb_dbstats_testdb_ok dbstats_testdb.
# TYPE mongodb_dbstats_testdb_ok untyped
mongodb_dbstats_testdb_ok 1
# HELP mongodb_dbstats_testdb_storageSize dbstats_testdb.
# TYPE mongodb_dbstats_testdb_storageSize untyped
mongodb_dbstats_testdb_storageSize 12288
# HELP mongodb_dbstats_testdb_views dbstats_testdb.
# TYPE mongodb_dbstats_testdb_views untyped
mongodb_dbstats_testdb_views 0` +
		"\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_dbstats_testdb_avgObjSize",
		"mongodb_dbstats_testdb_collections",
		"mongodb_dbstats_testdb_dataSize",
		"mongodb_dbstats_testdb_indexSize",
		"mongodb_dbstats_testdb_indexes",
		"mongodb_dbstats_testdb_objects",
		"mongodb_dbstats_testdb_views",
		"mongodb_dbstats_testdb_storageSize",
		"mongodb_dbstats_testdb_ok",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
