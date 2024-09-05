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
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestCurrentopCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	_ = database.Drop(ctx) //nolint: errcheck

	defer func() {
		err := database.Drop(ctx)
		assert.NoError(t, err)
	}()
	ch := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		coll := "testcol_01"
		for j := 0; j < 100; j++ { //nolint:intrange // false positive
			_, err := database.Collection(coll).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
			assert.NoError(t, err)
		}
		ch <- struct{}{}
		_, _ = database.Collection(coll).Find(ctx, bson.M{"$where": "function() {return sleep(100)}"})
	}()

	ti := labelsGetterMock{}
	st := "0s"

	c := newCurrentopCollector(ctx, client, logrus.New(), false, ti, st)

	// Filter metrics by reason:
	// 1. The result will be different on different hardware
	// 2. Can't check labels like 'decs' and 'opid' because they don't return a known value for comparison
	// It looks like:
	// # HELP mongodb_currentop_query_uptime currentop_query.
	// # TYPE mongodb_currentop_query_uptime untyped
	// mongodb_currentop_query_uptime{collection="testcol_00",database="testdb",decs="conn6365",ns="testdb.testcol_00",op="insert",opid="448307"} 2524

	filter := []string{
		"mongodb_currentop_query_uptime",
	}

	<-ch

	time.Sleep(1 * time.Second)

	count := testutil.CollectAndCount(c, filter...)
	assert.True(t, count > 0)
	wg.Wait()
}
