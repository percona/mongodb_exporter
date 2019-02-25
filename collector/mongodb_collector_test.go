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
	"fmt"
	"os"
	"testing"

	"github.com/percona/exporter_shared/helpers"
	"github.com/stretchr/testify/assert"

	"github.com/prometheus/client_golang/prometheus"
)

func testMongoDBURL() string {
	if u := os.Getenv("TEST_MONGODB_URI"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

func TestCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping functional test")
	}

	collector := NewMongodbCollector(&MongodbCollectorOpts{
		URI:                      testMongoDBURL(),
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

	descriptors := make(map[string]struct{})
	var descriptorsCount int
	for d := range descCh {
		descriptors[d.String()] = struct{}{}
		descriptorsCount++
	}

	var metricsCount int
	var versionInfoFound bool
	for m := range metricCh {

		if _, ok := descriptors[m.Desc().String()]; ok {
			delete(descriptors, m.Desc().String())
		}

		m := helpers.ReadMetric(m)
		switch m.Name {
		case "mongodb_version_info":
			versionInfoFound = true
		}
		metricsCount++
	}

	var missingDescMsg string
	for k := range descriptors {
		missingDescMsg += fmt.Sprintf("- %s\n", k)
	}

	assert.True(t, len(descriptors) == 0, "Number of descriptors collected and described should be the same. "+
		"Got '%d' Descriptors from collector.Describe() and '%d' from collector.Collect().\n"+
		"Missing descriptors: \n%s", descriptorsCount, metricsCount, missingDescMsg)
	assert.True(t, versionInfoFound, "version info metric not found")
}
