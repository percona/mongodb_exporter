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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// upProbeTimeout bounds the ping mongodb_up is read from when the scrape's own budget is gone
// already. It comes out of the slack --web.timeout-offset holds back, a second by default, so
// the scrape still answers before Prometheus gives up on it. A healthy server answers a ping in
// microseconds, so one that outlives this has told us something.
const upProbeTimeout = 500 * time.Millisecond

// This collector is always enabled and collects general MongoDB connectivity status.
type generalCollector struct {
	ctx      context.Context
	base     *baseCollector
	nodeType mongoDBNodeType
}

// newGeneralCollector creates a collector for MongoDB connectivity status.
func newGeneralCollector(ctx context.Context, client *mongo.Client, nodeType mongoDBNodeType, logger *slog.Logger) *generalCollector {
	return &generalCollector{
		ctx:      ctx,
		nodeType: nodeType,
		base:     newBaseCollector(client, logger.With("collector", "general")),
	}
}

func (d *generalCollector) Describe(ch chan<- *prometheus.Desc) {
	// Deliberately not gated on the scrape's budget, unlike every other collector. A scrape
	// that spent it all before reaching here -- a replica set with no primary keeps the setup
	// commands waiting for one, and a secondary is enough to connect -- would otherwise carry
	// no mongodb_up at all, which reads as a stale series rather than as a target that is down.
	//
	// What the metric then says is settled by the probe in mongodbUpMetric, not by the spent
	// budget: a server that answers is up, whatever the scrape managed to collect.
	d.base.describe(ch, d.collect)
}

func (d *generalCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *generalCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "general")()
	ch <- mongodbUpMetric(d.ctx, d.base.client, d.nodeType, d.base.logger)
}

func mongodbUpMetric(ctx context.Context, client *mongo.Client, nodeType mongoDBNodeType, log *slog.Logger) prometheus.Metric { //nolint:ireturn
	var value float64
	var clusterRole mongoDBNodeType

	if client != nil {
		// A scrape that spent its whole budget before reaching here would report the zero off a
		// ping that never left the process: the context is done, so the driver fails it at once.
		// That is an assertion that MongoDB is down drawn from no evidence, and a deployment
		// where the setup commands never fit in the budget would make it permanent -- a healthy
		// server reported down for as long as it stays big. So the probe gets a deadline of its
		// own, and the zero means a ping that ran and did not come back.
		probeCtx := ctx
		if ctx.Err() != nil {
			var cancel context.CancelFunc

			probeCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), upProbeTimeout)
			defer cancel()
		}

		if err := client.Ping(probeCtx, readpref.PrimaryPreferred()); err == nil {
			value = 1
		} else {
			log.Error("error while checking mongodb connection, mongo_up will be set to 0", "error", err.Error())
		}
		switch nodeType { //nolint:exhaustive
		case typeShardServer:
			clusterRole = typeMongod
		default:
			clusterRole = nodeType
		}
	}

	labels := map[string]string{"cluster_role": string(clusterRole)}
	d := prometheus.NewDesc("mongodb_up", "Whether MongoDB is up.", nil, labels)

	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value)
}

var _ prometheus.Collector = (*generalCollector)(nil)
