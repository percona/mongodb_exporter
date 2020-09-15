package exporter

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
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
		`constLabels: {}, variableLabels: [lock_mode resource]}`

	p, err := makeLockMetric(m, lm)
	assert.NoError(t, err)

	// Fix description since labels don't have a specific order because they are stores in a map.
	pd := p.Desc().String()
	pd = strings.ReplaceAll(pd, "resource lock_mode", "lock_mode resource")

	assert.Equal(t, want, pd)
}

func TestAddLocksMetrics(t *testing.T) {
	buf, err := ioutil.ReadFile(filepath.Join("testdata/", "locks.json"))
	assert.NoError(t, err)

	var m bson.M
	err = json.Unmarshal(buf, &m)
	assert.NoError(t, err)

	var metrics []prometheus.Metric
	metrics = locksMetrics(m)

	desc := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		// Fix description since labels don't have a specific order because they are stores in a map.
		ms := metric.Desc().String()
		var m dto.Metric
		err := metric.Write(&m)
		assert.NoError(t, err)

		ms = strings.ReplaceAll(ms, "resource lock_mode", "lock_mode resource")
		desc = append(desc, ms)
	}

	sort.Strings(desc)
	want := []string{
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: [lock_mode resource]}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: [lock_mode resource]}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: [lock_mode resource]}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: [lock_mode resource]}",
		"Desc{fqName: \"mongodb_ss_locks_acquireCount\", help: \"mongodb_ss_locks_acquireCount\", constLabels: {}, variableLabels: [lock_mode resource]}",
		"Desc{fqName: \"mongodb_ss_locks_acquireWaitCount\", help: \"mongodb_ss_locks_acquireWaitCount\", constLabels: {}, variableLabels: [lock_mode resource]}",
		"Desc{fqName: \"mongodb_ss_locks_timeAcquiringMicros\", help: \"mongodb_ss_locks_timeAcquiringMicros\", constLabels: {}, variableLabels: [lock_mode resource]}",
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

		t.Run(tt.name, func(t *testing.T) {
			buf, err := ioutil.ReadFile(filepath.Join("testdata/", "get_diagnostic_data.json"))
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
