package exporter

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
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
	metrics = append(metrics, locksMetrics(m)...)

	desc := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		// Fix description since labels don't have a specific order because they are stores in a map.
		ms := metric.Desc().String()
		ms = strings.ReplaceAll(ms, "resource lock_mode", "lock_mode resource")
		desc = append(desc, ms)
	}
	sort.Strings(desc)
	want := []string{
		`Desc{fqName: "mongodb_ss_locks_acquireCount", help: "mongodb_ss_locks_acquireCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_acquireCount", help: "mongodb_ss_locks_acquireCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_acquireCount", help: "mongodb_ss_locks_acquireCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_acquireCount", help: "mongodb_ss_locks_acquireCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_acquireCount", help: "mongodb_ss_locks_acquireCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_acquireCount", help: "mongodb_ss_locks_acquireCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_acquireWaitCount", help: "mongodb_ss_locks_acquireWaitCount", constLabels: {}, variableLabels: [lock_mode resource]}`,
		`Desc{fqName: "mongodb_ss_locks_timeAcquiringMicros", help: "mongodb_ss_locks_timeAcquiringMicros", constLabels: {}, variableLabels: [lock_mode resource]}`,
	}

	assert.Equal(t, want, desc)
}
