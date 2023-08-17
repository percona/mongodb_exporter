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

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

const (
	dbName = "testdb"
)

func TestDBStatsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database(dbName)
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

	c := newDBStatsCollector(ctx, client, logrus.New(), false, ti, []string{dbName}, false)
	expected := strings.NewReader(`
	# HELP mongodb_dbstats_collections dbstats.
	# TYPE mongodb_dbstats_collections untyped
	mongodb_dbstats_collections{database="testdb"} 3
	# HELP mongodb_dbstats_indexes dbstats.
	# TYPE mongodb_dbstats_indexes untyped
	mongodb_dbstats_indexes{database="testdb"} 3
	# HELP mongodb_dbstats_objects dbstats.
	# TYPE mongodb_dbstats_objects untyped
	mongodb_dbstats_objects{database="testdb"} 30` + "\n")

	// Only look at metrics created by our activity
	filters := []string{
		"mongodb_dbstats_collections",
		"mongodb_dbstats_indexes",
		"mongodb_dbstats_objects",
	}
	err := testutil.CollectAndCompare(c, expected, filters...)
	assert.NoError(t, err)
}
