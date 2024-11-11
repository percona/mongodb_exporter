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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// measureCollectTime measures time taken for scrape by collector
func measureCollectTime(ch chan<- prometheus.Metric, exporter, collector string) func() {
	startTime := time.Now()
	timeToCollectDesc := prometheus.NewDesc(
		"collector_scrape_time_ms",
		"Time taken for scrape by collector",
		[]string{"exporter"},
		prometheus.Labels{"collector": collector}, // to have ID calculated correctly
	)

	return func() {
		scrapeTime := time.Since(startTime)
		scrapeMetric := prometheus.MustNewConstMetric(
			timeToCollectDesc,
			prometheus.GaugeValue,
			float64(scrapeTime.Milliseconds()),
			exporter)
		ch <- scrapeMetric
	}
}
