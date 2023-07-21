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
	database.Drop(ctx)       //nolint:errcheck
	defer database.Drop(ctx) //nolint:errcheck

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

	collection := []string{"testdb.testcol_00", "testdb.testcol_01", "testdb.testcol_02"}
	c := newIndexStatsCollector(ctx, client, logrus.New(), false, true, ti, collection)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_indexstats_accesses_ops indexstats.accesses.
# TYPE mongodb_indexstats_accesses_ops untyped
mongodb_indexstats_accesses_ops{collection="testcol_00",database="testdb",key_name="_id_"} 0
mongodb_indexstats_accesses_ops{collection="testcol_00",database="testdb",key_name="idx_01"} 0
mongodb_indexstats_accesses_ops{collection="testcol_01",database="testdb",key_name="_id_"} 0
mongodb_indexstats_accesses_ops{collection="testcol_01",database="testdb",key_name="idx_01"} 0
mongodb_indexstats_accesses_ops{collection="testcol_02",database="testdb",key_name="_id_"} 0
mongodb_indexstats_accesses_ops{collection="testcol_02",database="testdb",key_name="idx_01"} 0` +
		"\n")

	filter := []string{
		"mongodb_indexstats_accesses_ops",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}

func TestDescendingIndexOverride(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	ti := labelsGetterMock{}

	database := client.Database("testdb")
	database.Drop(ctx)       //nolint:errcheck
	defer database.Drop(ctx) //nolint:errcheck

	for i := 0; i < 3; i++ {
		collection := fmt.Sprintf("testcol_%02d", i)
		for j := 0; j < 10; j++ {
			_, err := database.Collection(collection).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
			assert.NoError(t, err)
		}

		descendingMod := mongo.IndexModel{Keys: bson.M{"f1": -1}}
		_, err := database.Collection(collection).Indexes().CreateOne(ctx, descendingMod)
		assert.NoError(t, err)

		ascendingMod := mongo.IndexModel{Keys: bson.M{"f1": 1}}
		_, err = database.Collection(collection).Indexes().CreateOne(ctx, ascendingMod)
		assert.NoError(t, err)
	}

	collection := []string{"testdb.testcol_00", "testdb.testcol_01", "testdb.testcol_02"}
	c := newIndexStatsCollector(ctx, client, logrus.New(), false, true, ti, collection)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
  # HELP mongodb_indexstats_accesses_ops indexstats.accesses.
  # TYPE mongodb_indexstats_accesses_ops untyped
  mongodb_indexstats_accesses_ops{collection="testcol_00",database="testdb",key_name="_id_"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_00",database="testdb",key_name="f1_1"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_00",database="testdb",key_name="f1_DESC"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_01",database="testdb",key_name="_id_"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_01",database="testdb",key_name="f1_1"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_01",database="testdb",key_name="f1_DESC"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_02",database="testdb",key_name="_id_"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_02",database="testdb",key_name="f1_1"} 0
  mongodb_indexstats_accesses_ops{collection="testcol_02",database="testdb",key_name="f1_DESC"} 0` + "\n")

	filter := []string{
		"mongodb_indexstats_accesses_ops",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
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
