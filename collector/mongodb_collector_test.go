// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"os"
	"testing"

	"github.com/percona/exporter_shared/helpers"
	"github.com/stretchr/testify/assert"

	"github.com/prometheus/client_golang/prometheus"
)

func testMongoDBURL() string {
	if u := os.Getenv("TEST_MONGODB_URL"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

func TestCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping functional test")
	}

	collector := NewMongodbCollector(&MongodbCollectorOpts{
		URI: testMongoDBURL(),
		CollectDatabaseMetrics:   true,
		CollectCollectionMetrics: true,
		CollectTopMetrics:        true,
		CollectIndexUsageStats:   true,
	})

	descCh := make(chan *prometheus.Desc)
	go func() {
		collector.Describe(descCh)
		close(descCh)
	}()
	metricCh := make(chan prometheus.Metric)
	go func() {
		collector.Collect(metricCh)
		close(metricCh)
	}()

	var descs int
	for range descCh {
		descs++
	}

	var metrics int
	var versionInfoFound bool
	for m := range metricCh {
		m := helpers.ReadMetric(m)
		switch m.Name {
		case "mongodb_mongod_version_info":
			versionInfoFound = true
		}
		metrics++
	}

	assert.Equalf(t, descs, metrics, "got %d descs and %d metrics", descs, metrics)
	assert.True(t, versionInfoFound, "version info metric not found")
}
