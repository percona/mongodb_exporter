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
	// It seems like this test needs the queries to continue running so that current oplog is not empty.
	// TODO: figure out how to restore this test.
	t.Skip()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx)

	defer func() {
		err := database.Drop(ctx)
		assert.NoError(t, err)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			coll := fmt.Sprintf("testcol_%02d", i)
			_, err := database.Collection(coll).InsertOne(ctx, bson.M{"f1": 1, "f2": "2"})
			assert.NoError(t, err)
		}
	}()

	ti := labelsGetterMock{}

	c := newCurrentopCollector(ctx, client, logrus.New(), false, ti)

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

	count := testutil.CollectAndCount(c, filter...)
	assert.True(t, count > 0)
	wg.Wait()
}
