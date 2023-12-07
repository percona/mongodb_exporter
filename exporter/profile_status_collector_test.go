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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestProfileCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx) //nolint

	defer func() {
		err := database.Drop(ctx)
		assert.NoError(t, err)
	}()

	// Enable database profiler https://www.mongodb.com/docs/manual/tutorial/manage-the-database-profiler/
	cmd := bson.M{"profile": 2}
	_ = database.RunCommand(ctx, cmd)

	ti := labelsGetterMock{}

	c := newProfileCollector(ctx, client, logrus.New(), false, ti, 30)

	expected := strings.NewReader(`
	# HELP mongodb_profile_slow_query_count profile_slow_query.
	# TYPE mongodb_profile_slow_query_count counter
	mongodb_profile_slow_query_count{database="admin"} 0
	mongodb_profile_slow_query_count{database="config"} 0
	mongodb_profile_slow_query_count{database="local"} 0
	mongodb_profile_slow_query_count{database="testdb"} 0` +
		"\n")

	filter := []string{
		"mongodb_profile_slow_query_count",
	}

	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
