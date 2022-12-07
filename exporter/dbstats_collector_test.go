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

	c := newDBStatsCollector(ctx, client, logrus.New(), false, ti, testDBs)
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
