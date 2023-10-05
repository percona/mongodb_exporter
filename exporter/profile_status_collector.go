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
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type profileCollector struct {
	ctx            context.Context
	base           *baseCollector
	compatibleMode bool
	topologyInfo   labelsGetter
	profiletimets  int
}

// newProfileCollector creates a collector for being processed queries.
func newProfileCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger,
	compatible bool, topology labelsGetter, profileTimeTS int,
) *profileCollector {
	return &profileCollector{
		ctx:            ctx,
		base:           newBaseCollector(client, logger),
		compatibleMode: compatible,
		topologyInfo:   topology,
		profiletimets:  profileTimeTS,
	}
}

func (d *profileCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *profileCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *profileCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "profile")()

	logger := d.base.logger
	client := d.base.client
	timeScrape := d.profiletimets

	databases, err := databases(d.ctx, client, nil, nil)
	if err != nil {
		errors.Wrap(err, "cannot get the database names list")
		return
	}

	// Now time + '--collector.profile-time-ts'
	ts := primitive.NewDateTimeFromTime(time.Now().Add(-time.Duration(time.Second * time.Duration(timeScrape))))

	labels := d.topologyInfo.baseLabels()

	// Get all slow queries from all databases
	cmd := bson.M{"ts": bson.M{"$gte": ts}}
	for _, db := range databases {
		res, err := client.Database(db).Collection("system.profile").CountDocuments(d.ctx, cmd)
		if err != nil {
			errors.Wrapf(err, "cannot read system.profile")
			break
		}
		labels["database"] = db

		m := primitive.M{"count": res}

		logger.Debug("profile response from MongoDB:")
		debugResult(logger, primitive.M{db: m})

		for _, metric := range makeMetrics("profile_slow_query", m, labels, d.compatibleMode) {
			ch <- metric
		}
	}
}
