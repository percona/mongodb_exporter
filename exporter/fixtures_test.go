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
// subtrees the tests below need.
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

// gatherMetrics exports the metrics the way the registry does at scrape time, so duplicated
// series and inconsistent descriptors fail the test instead of the scrape.
func gatherMetrics(t *testing.T, metrics []prometheus.Metric) map[string]*dto.MetricFamily {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(staticCollector(metrics)), "descriptors must be consistent")

	gatheredMetrics, err := reg.Gather()
	require.NoError(t, err, "metrics must be exported only once")

	metricsByName := make(map[string]*dto.MetricFamily, len(gatheredMetrics))
	for _, metric := range gatheredMetrics {
		metricsByName[metric.GetName()] = metric
	}

	return metricsByName
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

// valuesByLabel returns the value of every series of a metric family, keyed by the value the
// given label has on that series.
func valuesByLabel(family *dto.MetricFamily, name string) map[string]float64 {
	values := make(map[string]float64, len(family.GetMetric()))
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == name {
				values[label.GetValue()] = metric.GetUntyped().GetValue()
			}
		}
	}

	return values
}
