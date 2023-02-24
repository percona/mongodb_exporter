// mongodb_exporter
// Copyright (C) 2022 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
