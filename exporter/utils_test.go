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
	"strings"

	"github.com/percona/exporter_shared/helpers"
)

func filterMetrics(metrics []*helpers.Metric, filters []string) []*helpers.Metric {
	res := make([]*helpers.Metric, 0, len(metrics))

	for _, m := range metrics {
		m.Value = 0
		for _, filterName := range filters {
			if m.Name == filterName {
				res = append(res, m)

				break
			}
		}
	}

	return res
}

func getMetricNames(lines []string) map[string]bool {
	names := map[string]bool{}

	for _, line := range lines {
		if strings.HasPrefix(line, "# TYPE ") {
			m := strings.Split(line, " ")
			names[m[2]] = true
		}
	}

	return names
}

func filterMetricsWithLabels(metrics []*helpers.Metric, filters []string, labels map[string]string) []*helpers.Metric {
	res := make([]*helpers.Metric, 0, len(metrics))
	for _, m := range metrics {
		for _, filterName := range filters {
			if m.Name == filterName {
				validMetric := true
				for labelKey, labelValue := range labels {
					if m.Labels[labelKey] != labelValue {
						validMetric = false

						break
					}
				}
				if validMetric {
					res = append(res, m)
				}

				break
			}
		}
	}
	return res
}
