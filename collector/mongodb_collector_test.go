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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping integration test")
	}

	collector := NewMongodbCollector(MongodbCollectorOpts{URI: "mongodb://localhost:27017"})

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

	var descs, metrics int
	for range descCh {
		descs++
	}
	for range metricCh {
		metrics++
	}

	if descs != metrics {
		t.Errorf("got %d descs and %d metrics", descs, metrics)
	}
}
