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
	"context"
	"log/slog"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type replSetGetConfigCollector struct {
	ctx  context.Context
	base *baseCollector

	compatibleMode bool
	topologyInfo   labelsGetter
}

// newReplicationSetConfigCollector creates a collector for configuration of replication set.
func newReplicationSetConfigCollector(ctx context.Context, client *mongo.Client, logger *slog.Logger, compatible bool, topology labelsGetter) *replSetGetConfigCollector {
	return &replSetGetConfigCollector{
		ctx:  ctx,
		base: newBaseCollector(client, logger.With("collector", "replset_config")),

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
	defer measureCollectTime(ch, "mongodb", "replset_config")()

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
		logger.Error("cannot get replSetGetConfig", "error", err)

		return
	}

	config, ok := m["config"].(bson.M)
	if !ok {
		err := errors.Wrapf(errUnexpectedDataType, "%T for data field", m["config"])
		logger.Error("cannot decode getDiagnosticData", "error", err)

		return
	}
	m = config

	logger.Debug("replSetGetConfig result:")
	debugResult(logger, m)

	for _, metric := range makeMetrics("rs_cfg", m, d.topologyInfo.baseLabels(), d.compatibleMode) {
		ch <- metric
	}
}

var _ prometheus.Collector = (*replSetGetConfigCollector)(nil)
