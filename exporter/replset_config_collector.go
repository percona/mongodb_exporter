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

// const (
// 	replicationNotEnabled        = 76
// 	replicationNotYetInitialized = 94
// )

type replSetGetConfigCollector struct {
	ctx  context.Context
	base *baseCollector

	compatibleMode bool
	topologyInfo   labelsGetter
}

// newReplicationSetConfigCollector creates a collector for configuration of replication set.
func newReplicationSetConfigCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, compatible bool, topology labelsGetter) *replSetGetConfigCollector {
	return &replSetGetConfigCollector{
		ctx:  ctx,
		base: newBaseCollector(client, logger),

		compatibleMode: compatible,
		topologyInfo:   topology,
	}
}

func (d *replSetGetConfigCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *replSetGetConfigCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *replSetGetConfigCollector) collect(ch chan<- prometheus.Metric) {
	defer prometheus.MeasureCollectTime(ch, "mongodb", "replset_config")()

	logger := d.base.logger
	client := d.base.client

	cmd := bson.D{{Key: "replSetGetConfig", Value: "1"}}
	res := client.Database("admin").RunCommand(d.ctx, cmd)

	var m bson.M

	if err := res.Decode(&m); err != nil {
		if e, ok := err.(mongo.CommandError); ok { //nolint // https://github.com/percona/mongodb_exporter/pull/295#issuecomment-922874632
			if e.Code == replicationNotYetInitialized || e.Code == replicationNotEnabled {
				return
			}
		}
		logger.Errorf("cannot get replSetGetConfig: %s", err)

		return
	}

	config, ok := m["config"].(bson.M)
	if !ok {
		err := errors.Wrapf(errUnexpectedDataType, "%T for data field", m["config"])
		logger.Errorf("cannot decode getDiagnosticData: %s", err)

		return
	}
	m = config

	logger.Debug("replSetGetConfig result:")
	debugResult(logger, m)

	for _, metric := range makeMetrics("cfg", m, d.topologyInfo.baseLabels(), d.compatibleMode) {
		ch <- metric
	}
}

var _ prometheus.Collector = (*replSetGetConfigCollector)(nil)
