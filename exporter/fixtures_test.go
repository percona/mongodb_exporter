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
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// diagnosticData83FixturePath is a getDiagnosticData reply captured from the MongoDB 8.3.2
// instance of https://github.com/percona/mongodb_exporter/issues/1285, trimmed to the
// subtrees the tests below need: serverStatus.opLatencies for the "histogram" bucket arrays
// and serverStatus.metrics.query for the "histograms" nodes.
const diagnosticData83FixturePath = "testdata/get_diagnostic_data_8.3.json"

// loadDiagnosticData83Fixture reads the captured command reply. Extended JSON is used instead of
// encoding/json because the driver decodes arrays into primitive.A while encoding/json
// produces []any, which makeMetrics silently walks past. A fixture parsed the plain way
// hides everything array shaped, histogram buckets included.
func loadDiagnosticData83Fixture(t *testing.T) bson.M {
	t.Helper()

	buf, err := os.ReadFile(diagnosticData83FixturePath)
	require.NoError(t, err)

	var m bson.M
	require.NoError(t, bson.UnmarshalExtJSON(buf, false, &m))

	return m
}

// gatherFamilies exports the metrics the way the registry does at scrape time, so
// duplicated series and inconsistent descriptors fail the test instead of the scrape.
func gatherFamilies(t *testing.T, metrics []prometheus.Metric) []*dto.MetricFamily {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(staticCollector(metrics)), "descriptors must be consistent")

	families, err := reg.Gather()
	require.NoError(t, err, "metrics must be exported only once")

	return families
}

// gatherMetrics exports the metrics and indexes the families by name.
func gatherMetrics(t *testing.T, metrics []prometheus.Metric) map[string]*dto.MetricFamily {
	t.Helper()

	families := gatherFamilies(t, metrics)

	metricsByName := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		metricsByName[family.GetName()] = family
	}

	return metricsByName
}

// gatheredMetricNames returns the exported metric names, in the order Gather sorts them.
func gatheredMetricNames(t *testing.T, metrics []prometheus.Metric) []string {
	t.Helper()

	families := gatherFamilies(t, metrics)

	names := make([]string, 0, len(families))
	for _, family := range families {
		names = append(names, family.GetName())
	}

	return names
}

// labelValues returns the value of the given label for every series of a metric family.
func labelValues(family *dto.MetricFamily, name string) []string {
	values := make([]string, 0, len(family.GetMetric()))
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == name {
				values = append(values, label.GetValue())
			}
		}
	}

	return values
}

// countsByLowerBound maps each bucket bound of a histogram family to the count of its series.
func countsByLowerBound(family *dto.MetricFamily) map[string]float64 {
	values := make(map[string]float64, len(family.GetMetric()))
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == histogramBoundLabel {
				values[label.GetValue()] = metric.GetCounter().GetValue()
			}
		}
	}

	return values
}
