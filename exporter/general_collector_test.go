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
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestGeneralCollector(t *testing.T) {
	t.Parallel()
	t.Run("mongod cluster role", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		client := tu.DefaultTestClient(ctx, t)
		nodeType, _ := getNodeType(ctx, client)
		c := newGeneralCollector(ctx, client, nodeType, promslog.New(&promslog.Config{}))

		filter := []string{
			"collector_scrape_time_ms",
		}
		count := testutil.CollectAndCount(c, filter...)
		assert.Equal(t, len(filter), count, "Meta-metric for collector is missing")

		// The last \n at the end of this string is important
		expected := strings.NewReader(`
	# HELP mongodb_up Whether MongoDB is up.
	# TYPE mongodb_up gauge
	mongodb_up {cluster_role="mongod"} 1
	` + "\n")
		filter = []string{
			"mongodb_up",
		}
		err := testutil.CollectAndCompare(c, expected, filter...)
		require.NoError(t, err)

		assert.NoError(t, client.Disconnect(ctx))

		expected = strings.NewReader(`
	# HELP mongodb_up Whether MongoDB is up.
	# TYPE mongodb_up gauge
	mongodb_up {cluster_role="mongod"} 0
	` + "\n")
		filter = []string{
			"mongodb_up",
		}
		err = testutil.CollectAndCompare(c, expected, filter...)
		require.NoError(t, err)
	})

	t.Run("mongos cluster role", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		port, err := tu.PortForContainer("mongos")
		require.NoError(t, err)
		client := tu.TestClient(ctx, port, t)

		nodeType, _ := getNodeType(ctx, client)
		c := newGeneralCollector(ctx, client, nodeType, promslog.New(&promslog.Config{}))

		filter := []string{
			"collector_scrape_time_ms",
		}
		count := testutil.CollectAndCount(c, filter...)
		assert.Equal(t, len(filter), count, "Meta-metric for collector is missing")

		// The last \n at the end of this string is important
		expected := strings.NewReader(`
	# HELP mongodb_up Whether MongoDB is up.
	# TYPE mongodb_up gauge
	mongodb_up {cluster_role="mongos"} 1
	` + "\n")
		filter = []string{
			"mongodb_up",
		}
		err = testutil.CollectAndCompare(c, expected, filter...)
		require.NoError(t, err)

		assert.NoError(t, client.Disconnect(ctx))

		expected = strings.NewReader(`
	# HELP mongodb_up Whether MongoDB is up.
	# TYPE mongodb_up gauge
	mongodb_up {cluster_role="mongos"} 0
	` + "\n")
		filter = []string{
			"mongodb_up",
		}
		err = testutil.CollectAndCompare(c, expected, filter...)
		require.NoError(t, err)
	})
}
