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
	"go.mongodb.org/mongo-driver/bson"

	"github.com/Percona-Lab/mnogo_exporter/internal/tu"
)

func TestCollStatsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx) //nolint
	collection := database.Collection("testcol")
	_, err := collection.InsertOne(ctx, bson.M{"f1": 1, "f2": "2"})
	assert.NoError(t, err)

	c := &collstatsCollector{
		ctx:         ctx,
		client:      client,
		collections: []string{"testdb.testcol"},
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_count count
# TYPE mongodb_count untyped
mongodb_count 1
# HELP mongodb_indexDetails_id_LSM_bloom_filter_hits indexDetails._id_.LSM.
# TYPE mongodb_indexDetails_id_LSM_bloom_filter_hits untyped
mongodb_indexDetails_id_LSM_bloom_filter_hits 0
# HELP mongodb_indexDetails_id_btree_overflow_pages indexDetails._id_.btree.
# TYPE mongodb_indexDetails_id_btree_overflow_pages untyped
mongodb_indexDetails_id_btree_overflow_pages 0` + "\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_count",
		"mongodb_indexDetails_id_LSM_bloom_filter_hits",
		"mongodb_indexDetails_id_btree_overflow_pages",
	}
	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
