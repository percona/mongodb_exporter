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

type documentdbDataCollector struct {
	client          *mongo.Client
	logger          *logrus.Logger
	topologyInfo    labelsGetter
	compatibleMode  bool
	notInReplicaset bool
	ctx             context.Context
}

func (d *documentdbDataCollector) inReplicaset() bool {
	return !d.notInReplicaset
}

func (d *documentdbDataCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *documentdbDataCollector) collect() (bson.M, error) {
	// serverStatus
	var serverStatus bson.M

	cmd := bson.D{{Key: "serverStatus", Value: "1"}}
	res := d.client.Database("admin").RunCommand(d.ctx, cmd)

	if err := res.Decode(&serverStatus); err != nil {
		d.logger.Errorf("cannot run serverStatus: %s", err)

		return nil, errors.Wrap(err, "cannot get serverStatus")
	}

	// replSetGetStatus
	var rsStatus bson.M

	cmd = bson.D{{Key: "replSetGetStatus", Value: "1"}}
	if d.inReplicaset() {
		res = d.client.Database("admin").RunCommand(d.ctx, cmd)

		if err := res.Decode(&rsStatus); err != nil {
			d.logger.Errorf("cannot get replSetGetStatus: %s", err)
			// Prevent running this command again. We already know we are not is a replicaset.
			d.notInReplicaset = true
		}
	}

	// local.oplog.rs.stats
	var oplogRsStats bson.M

	cmd = bson.D{{Key: "collStats", Value: "oplog.rs"}}
	if d.inReplicaset() {
		res = d.client.Database("local").RunCommand(d.ctx, cmd)

		if err := res.Decode(&oplogRsStats); err != nil {
			d.logger.Errorf("cannot get collStats for oplog.rs: %s", err)
		}
	}

	resp := bson.M{
		"data": bson.M{
			"serverStatus":         serverStatus,
			"replSetGetStatus":     rsStatus,
			"local.oplog.rs.stats": oplogRsStats,
		},
	}

	return resp, nil
}

func (d *documentdbDataCollector) Collect(ch chan<- prometheus.Metric) {
	m, err := d.collect()
	if err != nil {
		d.logger.Errorf("cannot collect documentDB diagnostic data: %s", err)

		return
	}

	metrics := makeMetrics("", m, d.topologyInfo.baseLabels(), d.compatibleMode)
	metrics = append(metrics, locksMetrics(m)...)

	if d.compatibleMode {
		metrics = append(metrics, specialMetrics(d.ctx, d.client, m, d.logger)...)

		if cem, err := cacheEvictedTotalMetric(m); err == nil {
			metrics = append(metrics, cem)
		}

		nodeType, err := getNodeType(d.ctx, d.client)
		if err != nil {
			d.logger.Errorf("Cannot get node type to check if this is a mongos: %s", err)
		} else if nodeType == typeMongos {
			metrics = append(metrics, mongosMetrics(d.ctx, d.client, d.logger)...)
		}
	}

	for _, metric := range metrics {
		ch <- metric
	}
}

// check interface.
var _ prometheus.Collector = (*documentdbDataCollector)(nil)
