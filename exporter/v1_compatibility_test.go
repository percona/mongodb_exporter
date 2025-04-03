// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestWalkTo(t *testing.T) {
	m := bson.M{
		"serverStatus": bson.M{
			"locks": bson.M{
				"ParallelBatchWriterMode": bson.M{
					"acquireCount": bson.M{
						"r": float64(1.23),
					},
				},
			},
		},
	}

	testCases := []struct {
		path []string
		want interface{}
	}{
		{
			path: []string{"serverStatus", "locks", "ParallelBatchWriterMode", "acquireCount", "r"},
			want: float64(1.23),
		},
		{
			path: []string{"serverStatus", "locks", "ParallelBatchWriterMode", "acquireCount", "r", "w"},
			want: nil,
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, walkTo(m, tc.path), tc.want)
	}
}

func TestMakeLockMetric(t *testing.T) {
	m := bson.M{
		"serverStatus": bson.M{
			"locks": bson.M{
				"ParallelBatchWriterMode": bson.M{
					"acquireCount": bson.M{
						"r": float64(1.23),
					},
				},
			},
		},
	}

	lm := lockMetric{
		name:   "mongodb_ss_locks_acquireCount",
		path:   strings.Split("serverStatus_locks_ParallelBatchWriterMode_acquireCount_r", "_"),
		labels: map[string]string{"lock_mode": "r", "resource": "ParallelBatchWriterMode"},
	}

	want := `Desc{fqName: "mongodb_ss_locks_acquireCount", ` +
		`help: "mongodb_ss_locks_acquireCount", ` +
		`constLabels: {}, variableLabels: {lock_mode,resource}}`

	p, err := makeLockMetric(m, lm)
	assert.NoError(t, err)

	// Fix description since labels don't have a specific order because they are stores in a map.
	pd := p.Desc().String()
	pd = strings.ReplaceAll(pd, "resource,lock_mode", "lock_mode,resource")

	assert.Equal(t, want, pd)
}

func TestAddLocksMetrics(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata/", "locks.json"))
	assert.NoError(t, err)

	var m bson.M
	err = json.Unmarshal(buf, &m)
	assert.NoError(t, err)

	logLevel := promslog.NewLevel()
	err = logLevel.Set("debug")
	require.NoError(t, err)
	logger := promslog.New(&promslog.Config{
		Level: logLevel,
	})
	metrics := locksMetrics(logger.With("component", "test"), m)

	desc := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		// Fix description since labels don't have a specific order because they are stores in a map.
		ms := metric.Desc().String()
		var m dto.Metric
		err := metric.Write(&m)
		assert.NoError(t, err)

		ms = strings.ReplaceAll(ms, "resource,lock_mode", "lock_mode,resource")
		desc = append(desc, ms)
	}

	sort.Strings(desc)
	want := []string{
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: {lock_mode,resource}}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: {lock_mode,resource}}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: {lock_mode,resource}}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: {lock_mode,resource}}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: {lock_mode,resource}}",
		"Desc{fqName: \"mongodb_ss_locks_acquireWaitCount\", help: \"mongodb_ss_locks_acquireWaitCount\", constLabels: {}, variableLabels: {lock_mode,resource}}",
		"Desc{fqName: \"mongodb_ss_locks_timeAcquiringMicros\", help: \"mongodb_ss_locks_timeAcquiringMicros\", constLabels: {}, variableLabels: {lock_mode,resource}}",
	}

	assert.Equal(t, want, desc)
}

func TestSumMetrics(t *testing.T) {
	tests := []struct {
		name     string
		paths    [][]string
		expected float64
	}{
		{
			name: "timeAcquire",
			paths: [][]string{
				{"serverStatus", "locks", "Global", "timeAcquiringMicros", "W"},
				{"serverStatus", "locks", "Global", "timeAcquiringMicros", "w"},
			},
			expected: 42361,
		},
		{
			name: "timeAcquire",
			paths: [][]string{
				{"serverStatus", "locks", "Global", "acquireCount", "r"},
				{"serverStatus", "locks", "Global", "acquireCount", "w"},
			},
			expected: 158671,
		},
	}
	for _, tt := range tests {
		testCase := tt

		t.Run(testCase.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata/", "get_diagnostic_data.json"))
			assert.NoError(t, err)

			var m bson.M
			err = json.Unmarshal(buf, &m)
			assert.NoError(t, err)

			sum, err := sumMetrics(m, testCase.paths)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, sum)
		})
	}
}

func TestCreateOldMetricFromNew(t *testing.T) {
	rm := &rawMetric{
		// Full Qualified Name
		fqName: "mongodb_ss_globalLock_activeClients_mmm",
		help:   "mongodb_ss_globalLock_activeClients_mmm",
		ln:     []string{},
		lv:     []string{},
		val:    1,
		vt:     prometheus.UntypedValue,
	}
	c := conversion{
		oldName:     "mongodb_mongod_global_lock_client",
		prefix:      "mongodb_ss_globalLock_activeClients",
		suffixLabel: "type",
	}

	want := &rawMetric{
		fqName: "mongodb_mongod_global_lock_client",
		help:   "mongodb_mongod_global_lock_client",
		ln:     []string{"type"},
		lv:     []string{"mmm"}, // suffix is being converted. no mapping
		val:    1,
		vt:     3,
	}
	nm := createOldMetricFromNew(rm, c)
	assert.Equal(t, want, nm)
}

func TestMongosMetrics(t *testing.T) {
	t.Parallel()
	t.Run("test mongodb_mongos_sharding_chunks_is_balancer_running metric", func(t *testing.T) {
		type bss struct {
			Mode string `bson:"mode"`
		}

		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		port, err := tu.PortForContainer("mongos")
		require.NoError(t, err)
		client := tu.TestClient(ctx, port, t)

		var bs bss
		cmd := bson.D{{Key: "balancerStatus", Value: "1"}}
		err = client.Database("admin").RunCommand(ctx, cmd).Decode(&bs)
		require.NoError(t, err)

		var metric prometheus.Metric
		var m dto.Metric
		metric, err = balancerEnabled(ctx, client)
		assert.NoError(t, err)

		err = metric.Write(&m)
		assert.NoError(t, err)

		expected := 0
		if bs.Mode == "full" {
			expected = 1
		}
		assert.Equal(t, float64(expected), m.GetGauge().GetValue()) //nolint
	})
}

// myState should always return a metric. If there is no connection, the value
// should be the MongoDB unknown state = 6
func TestMyState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		containerName string
		allowedStates []float64
	}{
		{
			name:          "correctly gets state for data-carrying node",
			containerName: "mongo-1-1",
			allowedStates: []float64{float64(PrimaryState), float64(SecondaryState)},
		},
		{
			name:          "correctly gets state for arbiter node",
			containerName: "mongo-1-arbiter",
			allowedStates: []float64{float64(ArbiterState)},
		},
		{
			name:          "gets unknown state for standalone instance",
			containerName: "standalone",
			allowedStates: []float64{float64(UnknownState)},
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			port, err := tu.PortForContainer(testCase.containerName)
			require.NoError(t, err)
			client := tu.TestClient(ctx, port, t)
			var m dto.Metric

			metric := myState(ctx, client)
			err = metric.Write(&m)
			assert.NoError(t, err)
			assert.Contains(t, testCase.allowedStates, *m.Gauge.Value)

			err = client.Disconnect(ctx)
			assert.NoError(t, err)

			metric = myState(ctx, client)
			err = metric.Write(&m)
			assert.NoError(t, err)
			assert.Equal(t, float64(UnknownState), *m.Gauge.Value)
		})
	}
}

func TestArbiterMetrics(t *testing.T) {
	t.Parallel()
	t.Run("correctly gets member count from arbiter node", func(t *testing.T) {
		t.Parallel()
		containerName := "mongo-1-arbiter"

		logLevel := promslog.NewLevel()
		err := logLevel.Set("debug")
		require.NoError(t, err)
		logger := promslog.New(&promslog.Config{
			Level: logLevel,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		port, err := tu.PortForContainer(containerName)
		require.NoError(t, err)
		client := tu.TestClient(ctx, port, t)
		metrics := arbiterMetrics(ctx, client, logger.With("component", "test"))
		var rsMembers dto.Metric
		for _, m := range metrics {
			if strings.HasPrefix(m.Desc().String(), `Desc{fqName: "mongodb_mongod_replset_number_of_members"`) {
				err = m.Write(&rsMembers)
				assert.NoError(t, err)

				break
			}
		}
		assert.NoError(t, err)
		assert.Equal(t, float64(4), *rsMembers.Gauge.Value)

		err = client.Disconnect(ctx)
		assert.NoError(t, err)
	})
}
