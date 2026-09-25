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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestScrapeAllHostLabels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, prometheus.Labels{}, scrapeAllHostLabels(""))
	assert.Equal(t, prometheus.Labels{"mongo_instance": "node-a:27017"}, scrapeAllHostLabels("node-a:27017"))
}
