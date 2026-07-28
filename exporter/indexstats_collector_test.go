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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err)
		}
		mod := mongo.IndexModel{
			Keys: bson.M{
				"f1": 1,
			}, Options: &options.IndexOptions{
				Name: pointer.ToString("idx_01"),
			},
		}
		_, err := database.Collection(collection).Indexes().CreateOne(ctx, mod)
		require.NoError(t, err)
	}

	collection := []string{"testdb.testcol_00", "testdb.testcol_01", "testdb.testcol_02"}
	c := newIndexStatsCollector(ctx, client, promslog.New(&promslog.Config{}), false, true, ti, collection)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_indexstats_accesses_ops indexstats.accesses.ops
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
	require.NoError(t, err)
}

// Through mongos, a sharded collection reports one $indexStats document per
// shard for the same index. Without the shard label they all collapse into one
// series and the duplicates are dropped.
//
//nolint:paralleltest,funlen
func TestIndexStatsCollectorSharded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := tu.DefaultTestClientMongoS(ctx, t)

	dbName, collName := "testdb_indexstats_sharded", "testcol"
	namespace := dbName + "." + collName

	database := client.Database(dbName)
	database.Drop(ctx)       //nolint:errcheck
	defer database.Drop(ctx) //nolint:errcheck

	admin := client.Database("admin")

	var shardList struct {
		Shards []bson.M `bson:"shards"`
	}
	if err := admin.RunCommand(ctx, bson.D{{Key: "listShards", Value: 1}}).Decode(&shardList); err != nil {
		t.Skipf("cannot list shards: %v", err)
	}
	if len(shardList.Shards) < 2 {
		t.Skipf("the test cluster has %d shards, at least 2 are needed", len(shardList.Shards))
	}

	if err := admin.RunCommand(ctx, bson.D{{Key: "enableSharding", Value: dbName}}).Err(); err != nil {
		t.Skipf("cannot enable sharding on %s: %v", dbName, err)
	}

	// A hashed shard key on an empty collection presplits the initial chunks and
	// spreads them over all shards, so every shard owns chunks of the collection.
	shardCmd := bson.D{
		{Key: "shardCollection", Value: namespace},
		{Key: "key", Value: bson.D{{Key: "_id", Value: "hashed"}}},
	}
	if err := admin.RunCommand(ctx, shardCmd).Err(); err != nil {
		t.Skipf("cannot shard %s: %v", namespace, err)
	}

	docs := make([]any, 0, 100)
	for i := 0; i < 100; i++ {
		docs = append(docs, bson.M{"f1": i})
	}
	_, err := database.Collection(collName).InsertMany(ctx, docs)
	require.NoError(t, err)

	c := newIndexStatsCollector(ctx, client, promslog.New(&promslog.Config{}), false, false, labelsGetterMock{}, []string{namespace})

	// Register runs Describe, which collects everything, and rejects a metric
	// name described with two different label sets. That is how a shard label
	// set for only some of the documents would surface.
	registry := prometheus.NewPedanticRegistry()
	require.NoError(t, registry.Register(c))

	families, err := registry.Gather()
	require.NoError(t, err)

	shardsByIndex := make(map[string][]string)
	for _, family := range families {
		if family.GetName() != "mongodb_indexstats_accesses_ops" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			require.Equal(t, dbName, labels["database"])
			require.Equal(t, collName, labels["collection"])
			require.NotEmpty(t, labels["shard"], "series without a shard label: %v", labels)

			indexName := labels["key_name"]
			shardsByIndex[indexName] = append(shardsByIndex[indexName], labels["shard"])
		}
	}

	require.Contains(t, shardsByIndex, "_id_")
	for indexName, shards := range shardsByIndex {
		require.Len(t, slices.Compact(slices.Sorted(slices.Values(shards))), len(shards),
			"index %s exposes the same shard more than once: %v", indexName, shards)
		require.Greater(t, len(shards), 1, "index %s is exposed for a single shard only: %v", indexName, shards)
	}
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
			require.NoError(t, err)
		}

		descendingMod := mongo.IndexModel{Keys: bson.M{"f1": -1}}
		_, err := database.Collection(collection).Indexes().CreateOne(ctx, descendingMod)
		require.NoError(t, err)

		ascendingMod := mongo.IndexModel{Keys: bson.M{"f1": 1}}
		_, err = database.Collection(collection).Indexes().CreateOne(ctx, ascendingMod)
		require.NoError(t, err)
	}

	collection := []string{"testdb.testcol_00", "testdb.testcol_01", "testdb.testcol_02"}
	c := newIndexStatsCollector(ctx, client, promslog.New(&promslog.Config{}), false, true, ti, collection)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
  # HELP mongodb_indexstats_accesses_ops indexstats.accesses.ops
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
	require.NoError(t, err)
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
		require.Equal(t, want, got)
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
		require.Equal(t, want, got)
	})
}
