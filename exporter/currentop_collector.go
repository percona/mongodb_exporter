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
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type currentopCollector struct {
	ctx               context.Context
	base              *baseCollector
	compatibleMode    bool
	topologyInfo      labelsGetter
	currentopslowtime string
}

var ErrInvalidOrMissingInprogEntry = errors.New("invalid or missing inprog entry in currentop results")

// newCurrentopCollector creates a collector for being processed queries.
func newCurrentopCollector(ctx context.Context, client *mongo.Client, logger *slog.Logger,
	compatible bool, topology labelsGetter, currentOpSlowTime string,
) *currentopCollector {
	return &currentopCollector{
		ctx:               ctx,
		base:              newBaseCollector(client, logger.With("collector", "currentop")),
		compatibleMode:    compatible,
		topologyInfo:      topology,
		currentopslowtime: currentOpSlowTime,
	}
}

func (d *currentopCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *currentopCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *currentopCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "currentop")()

	logger := d.base.logger
	client := d.base.client

	slowQueriesEnabled := false
	var slowtimems int64

	if d.currentopslowtime != "" {
		slowtime, err := time.ParseDuration(d.currentopslowtime)
		if err != nil {
			logger.Error("Failed to parse currentop slowtime, slow query metrics disabled", "value", d.currentopslowtime, "error", err)
		} else {
			slowQueriesEnabled = true
			slowtimems = slowtime.Microseconds()
		}
	}

	cmd := bson.D{{Key: "currentOp", Value: true}}

	if slowQueriesEnabled {
		// Get all requests that are being processed except system requests (admin and local).
		cmd = append(cmd,
			bson.E{Key: "active", Value: true},
			bson.E{Key: "op", Value: bson.D{{Key: "$ne", Value: ""}}},
			bson.E{Key: "ns", Value: bson.D{
				{Key: "$ne", Value: ""},
				{Key: "$not", Value: bson.D{{Key: "$regex", Value: "^admin.*|^local.*"}}},
			}},
			bson.E{Key: "microsecs_running", Value: bson.D{
				{Key: "$exists", Value: true},
				{Key: "$gte", Value: slowtimems},
			}},
		)
	}

	res := client.Database("admin").RunCommand(d.ctx, cmd)

	var r primitive.M
	if err := res.Decode(&r); err != nil {
		logger.Error("Failed to decode currentOp response", "error", err)
		ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
		return
	}

	logger.Debug("currentop response from MongoDB:")
	debugResult(logger, r)

	if slowQueriesEnabled {
		d.collectSlowQueries(ch, r)
	}

	currentOpFsyncLockStateDesc := prometheus.NewDesc(
		"mongodb_currentop_fsync_lock_state",
		"Whether MongoDB is currently fsync locked (1 = locked, 0 = unlocked)",
		nil,
		nil,
	)

	fsyncIsLocked := 0.0
	if v, ok := r["fsyncLock"].(bool); ok && v {
		fsyncIsLocked = 1.0
	}
	ch <- prometheus.MustNewConstMetric(currentOpFsyncLockStateDesc, prometheus.GaugeValue, fsyncIsLocked)
}

// collectSlowQueries extracts and emits metrics for slow queries from the currentOp response.
func (d *currentopCollector) collectSlowQueries(ch chan<- prometheus.Metric, r primitive.M) {
	logger := d.base.logger

	inprog, ok := r["inprog"].(primitive.A)
	if !ok {
		logger.Error(fmt.Sprintf("Invalid type primitive.A assertion for 'inprog': %T", r["inprog"]))
		ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(ErrInvalidOrMissingInprogEntry),
			ErrInvalidOrMissingInprogEntry)
		return
	}

	labels := d.topologyInfo.baseLabels()
	ln := []string{"opid", "op", "desc", "database", "collection", "ns"}
	const name = "mongodb_currentop_query_uptime"
	pd := prometheus.NewDesc(name, " mongodb_currentop_query_uptime currentop_query", ln, labels)

	for _, bsonMap := range inprog {
		bsonMapElement, ok := bsonMap.(primitive.M)
		if !ok {
			logger.Error(fmt.Sprintf("Invalid type primitive.M assertion for bsonMap: %T", bsonMapElement))

			continue
		}
		opid, ok := bsonMapElement["opid"].(int32)
		if !ok {
			logger.Error(fmt.Sprintf("Invalid type int32 assertion for 'opid': %T", bsonMapElement))

			continue
		}
		namespace, ok := bsonMapElement["ns"].(string)
		if !ok {
			logger.Error(fmt.Sprintf("Invalid type string assertion for 'ns': %T", bsonMapElement))

			continue
		}
		db, collection := splitNamespace(namespace)
		op, ok := bsonMapElement["op"].(string)
		if !ok {
			logger.Error(fmt.Sprintf("Invalid type string assertion for 'op': %T", bsonMapElement))

			continue
		}
		desc, ok := bsonMapElement["desc"].(string)
		if !ok {
			logger.Error(fmt.Sprintf("Invalid type string assertion for 'desc': %T", bsonMapElement))

			continue
		}
		microsecsRunning, ok := bsonMapElement["microsecs_running"].(int64)
		if !ok {
			logger.Error(fmt.Sprintf("Invalid type int64 assertion for 'microsecs_running': %T", bsonMapElement))

			continue
		}

		lv := []string{strconv.Itoa(int(opid)), op, desc, db, collection, namespace}
		ch <- prometheus.MustNewConstMetric(pd, prometheus.GaugeValue, float64(microsecsRunning), lv...)
	}
}
