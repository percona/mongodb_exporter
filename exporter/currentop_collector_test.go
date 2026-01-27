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
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestCurrentopCollectorMetrics(t *testing.T) {
	ctx, cancel, client, database, adminDB := setupCurrentopTest(t)
	defer cancel()
	defer func() {
		err := database.Drop(ctx)
		assert.NoError(t, err)
	}()

	var wg sync.WaitGroup
	ch := make(chan struct{})
	wg.Add(1)
	go generateSlowOperation(ctx, t, &wg, ch, database)

	ti := labelsGetterMock{}
	st := "0s"
	c := newCurrentopCollector(
		ctx,
		client,
		promslog.New(&promslog.Config{}),
		false,
		ti,
		st,
	)

	<-ch
	time.Sleep(1 * time.Second)

	assertSlowQueryMetricsCollected(t, c)
	assertFsyncLockStateMetrics(t, ctx, c, adminDB)

	wg.Wait()
}

func setupCurrentopTest(t *testing.T) (context.Context, context.CancelFunc, *mongo.Client, *mongo.Database, *mongo.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	client := tu.DefaultTestClient(ctx, t)
	database := client.Database("testdb")
	_ = database.Drop(ctx)
	adminDB := client.Database("admin")
	return ctx, cancel, client, database, adminDB
}

func generateSlowOperation(ctx context.Context, t *testing.T, wg *sync.WaitGroup, ch chan struct{}, database *mongo.Database) {
	defer wg.Done()
	coll := "testcol_01"
	for j := 0; j < 100; j++ { //nolint:intrange // false positive
		_, err := database.Collection(coll).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
		assert.NoError(t, err)
	}
	ch <- struct{}{}
	_, _ = database.Collection(coll).Find(ctx, bson.M{"$where": "function() {return sleep(100)}"})
}

func assertSlowQueryMetricsCollected(t *testing.T, c *currentopCollector) {
	slowQueryMetrics := []string{
		"mongodb_currentop_query_uptime",
	}
	count := testutil.CollectAndCount(c, slowQueryMetrics...)
	assert.Positive(t, count)
}

func assertFsyncLockStateMetrics(t *testing.T, ctx context.Context, c *currentopCollector, adminDB *mongo.Database) {
	fsyncMetrics := []string{
		"mongodb_currentop_fsync_lock_state",
	}
	count := testutil.CollectAndCount(c, fsyncMetrics...)
	assert.Equal(t, 1, count)

	err := adminDB.RunCommand(ctx, bson.D{
		{Key: "fsync", Value: 1},
		{Key: "lock", Value: true},
	}).Err()
	assert.NoError(t, err)

	defer func() {
		_ = adminDB.RunCommand(ctx, bson.D{
			{Key: "fsyncUnlock", Value: 1},
		}).Err()
	}()

	time.Sleep(500 * time.Millisecond)
	count = testutil.CollectAndCount(c, fsyncMetrics...)
	assert.Equal(t, 1, count)

	err = adminDB.RunCommand(ctx, bson.D{
		{Key: "fsyncUnlock", Value: 1},
	}).Err()
	assert.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	count = testutil.CollectAndCount(c, fsyncMetrics...)
	assert.Equal(t, 1, count)
}
