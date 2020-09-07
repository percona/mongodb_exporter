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

	"github.com/AlekSi/pointer"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestIndexStatsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	ti := labelsGetterMock{}

	database := client.Database("testdb")
	database.Drop(ctx) //nolint
	for i := 0; i < 3; i++ {
		collection := fmt.Sprintf("testcol_%02d", i)
		for j := 0; j < 10; j++ {
			_, err := database.Collection(collection).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
			assert.NoError(t, err)
		}
		mod := mongo.IndexModel{
			Keys: bson.M{
				"f1": 1,
			}, Options: &options.IndexOptions{
				Name: pointer.ToString("idx_01"),
			},
		}
		_, err := database.Collection(collection).Indexes().CreateOne(ctx, mod)
		assert.NoError(t, err)
	}

	c := &indexstatsCollector{
		client:       client,
		collections:  []string{"testdb.testcol_00", "testdb.testcol_01", "testdb.testcol_02"},
		logger:       logrus.New(),
		topologyInfo: ti,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_testdb_testcol_00_id_accesses_ops testdb_testcol_00__id_.accesses.
# TYPE mongodb_testdb_testcol_00_id_accesses_ops untyped
mongodb_testdb_testcol_00_id_accesses_ops{key_name="_id_",namespace="testdb.testcol_00"} 0
# HELP mongodb_testdb_testcol_00_idx_01_accesses_ops testdb_testcol_00_idx_01.accesses.
# TYPE mongodb_testdb_testcol_00_idx_01_accesses_ops untyped
mongodb_testdb_testcol_00_idx_01_accesses_ops{key_name="idx_01",namespace="testdb.testcol_00"} 0
# HELP mongodb_testdb_testcol_01_id_accesses_ops testdb_testcol_01__id_.accesses.
# TYPE mongodb_testdb_testcol_01_id_accesses_ops untyped
mongodb_testdb_testcol_01_id_accesses_ops{key_name="_id_",namespace="testdb.testcol_01"} 0
# HELP mongodb_testdb_testcol_01_idx_01_accesses_ops testdb_testcol_01_idx_01.accesses.
# TYPE mongodb_testdb_testcol_01_idx_01_accesses_ops untyped
mongodb_testdb_testcol_01_idx_01_accesses_ops{key_name="idx_01",namespace="testdb.testcol_01"} 0
# HELP mongodb_testdb_testcol_02_id_accesses_ops testdb_testcol_02__id_.accesses.
# TYPE mongodb_testdb_testcol_02_id_accesses_ops untyped
mongodb_testdb_testcol_02_id_accesses_ops{key_name="_id_",namespace="testdb.testcol_02"} 0
# HELP mongodb_testdb_testcol_02_idx_01_accesses_ops testdb_testcol_02_idx_01.accesses.
# TYPE mongodb_testdb_testcol_02_idx_01_accesses_ops untyped
mongodb_testdb_testcol_02_idx_01_accesses_ops{key_name="idx_01",namespace="testdb.testcol_02"} 0` + "\n")

	err := testutil.CollectAndCompare(c, expected)
	assert.NoError(t, err)
}

func TestSanitize(t *testing.T) {
	t.Run("With building", func(t *testing.T) {
		in := bson.M{
			"accesses": bson.M{
				"ops":   3,
				"since": "2020-08-10T16:34:52.4-03:00",
			},
			"host": "7ba0382b199b:27017",
			"key": bson.M{
				"f1": 1,
			},
			"name": "idx_01",
			"spec": bson.M{
				"key": bson.M{
					"f1": 1,
				},
				"name": "idx_01",
				"ns":   "testdb.testcol_01",
				"v":    2,
			},
			"building": 1,
		}
		want := primitive.M{
			"accesses": primitive.M{
				"ops": float64(3),
			},
			"building": float64(1),
		}
		got := sanitizeMetrics(in)
		assert.Equal(t, want, got)
	})

	t.Run("Without building", func(t *testing.T) {
		in := bson.M{
			"accesses": bson.M{
				"ops":   3,
				"since": "2020-08-10T16:34:52.4-03:00",
			},
			"host": "7ba0382b199b:27017",
			"key": bson.M{
				"f1": 1,
			},
			"name": "idx_01",
			"spec": bson.M{
				"key": bson.M{
					"f1": 1,
				},
				"name": "idx_01",
				"ns":   "testdb.testcol_01",
				"v":    2,
			},
		}
		want := primitive.M{
			"accesses": primitive.M{
				"ops": float64(3),
			},
		}
		got := sanitizeMetrics(in)
		assert.Equal(t, want, got)
	})
}
