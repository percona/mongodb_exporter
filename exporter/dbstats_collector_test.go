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

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestDBStatsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	setupDB(ctx, t, client)
	defer cleanupDB(ctx, client)

	ti := labelsGetterMock{}

	c := newDBStatsCollector(ctx, client, logrus.New(), false, ti, testDBs, false)
	expected := strings.NewReader(`
	# HELP mongodb_dbstats_collections dbstats.
	# TYPE mongodb_dbstats_collections untyped
	mongodb_dbstats_collections{database="testdb01"} 5
	mongodb_dbstats_collections{database="testdb02"} 5
	# HELP mongodb_dbstats_indexes dbstats.
	# TYPE mongodb_dbstats_indexes untyped
	mongodb_dbstats_indexes{database="testdb01"} 4
	mongodb_dbstats_indexes{database="testdb02"} 4
	# HELP mongodb_dbstats_objects dbstats.
	# TYPE mongodb_dbstats_objects untyped
	mongodb_dbstats_objects{database="testdb01"} 80
	mongodb_dbstats_objects{database="testdb02"} 80` + "\n")

	// Only look at metrics created by our activity
	filters := []string{
		"mongodb_dbstats_collections",
		"mongodb_dbstats_indexes",
		"mongodb_dbstats_objects",
	}
	err := testutil.CollectAndCompare(c, expected, filters...)
	assert.NoError(t, err)
}
