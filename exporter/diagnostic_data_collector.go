// mongodb_exporter
// Copyright (C) 2017 Percona LLC
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
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type diagnosticDataCollector struct {
	ctx            context.Context
	client         *mongo.Client
	compatibleMode bool
	logger         *logrus.Logger
	topologyInfo   labelsGetter
}

func (d *diagnosticDataCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *diagnosticDataCollector) Collect(ch chan<- prometheus.Metric) {
	var m bson.M

	cmd := bson.D{{Key: "getDiagnosticData", Value: "1"}}
	res := d.client.Database("admin").RunCommand(d.ctx, cmd)

	if err := res.Decode(&m); err != nil {
		d.logger.Errorf("cannot run getDiagnosticData: %s", err)
		return
	}

	m, ok := m["data"].(bson.M)
	if !ok {
		err := errors.Wrapf(errUnexpectedDataType, "%T for data field", m["data"])
		d.logger.Errorf("cannot decode getDiagnosticData: %s", err)
		return
	}

	d.logger.Debug("getDiagnosticData result")
	debugResult(d.logger, m)

	metrics := makeMetrics("", m, d.topologyInfo.baseLabels(), d.compatibleMode)
	metrics = append(metrics, locksMetrics(m)...)

	// PMM dashboards looks for this metric so, in compatibility mode, we must expose it.
	// FIXME Add it in both modes: https://jira.percona.com/browse/PMM-6585
	if d.compatibleMode {
		metrics = append(metrics, mongodbUpMetric())
	}

	for _, metric := range metrics {
		ch <- metric
	}
}

// check interface.
var _ prometheus.Collector = (*diagnosticDataCollector)(nil)
