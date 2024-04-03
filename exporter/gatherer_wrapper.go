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
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

// GathererWrapped is a wrapper for prometheus.Gatherer that adds labels to all metrics.
type GathererWrapped struct {
	originalGatherer prometheus.Gatherer
	labels           prometheus.Labels
}

// NewGathererWrapper creates a new GathererWrapped with the given Gatherer and additional labels.
func NewGathererWrapper(gs prometheus.Gatherer, labels prometheus.Labels) *GathererWrapped {
	return &GathererWrapped{
		originalGatherer: gs,
		labels:           labels,
	}
}

// Gather implements prometheus.Gatherer interface.
func (g *GathererWrapped) Gather() ([]*io_prometheus_client.MetricFamily, error) {

	metrics, err := g.originalGatherer.Gather()
	if err != nil {
		return nil, err
	}

	for _, metric := range metrics {
		for _, m := range metric.Metric {
			for k, v := range g.labels {
				m.Label = append(m.Label, &io_prometheus_client.LabelPair{
					Name:  &k,
					Value: &v,
				})
			}
		}
	}

	return metrics, nil
}
