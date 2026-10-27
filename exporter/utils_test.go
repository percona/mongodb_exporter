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
	"time"

	"github.com/percona/exporter_shared/helpers"
)

// testTimeout bounds a whole test: connecting to the cluster, building its
// fixtures and asserting on them. It matches the driver's default server
// selection timeout, so an unreachable server surfaces the driver's own
// diagnostic rather than a bare context deadline. A genuine hang is caught by
// the -timeout the Makefile passes to go test.
const testTimeout = 30 * time.Second

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
