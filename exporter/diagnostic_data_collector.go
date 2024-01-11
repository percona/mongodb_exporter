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

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	kmipEncryption         = "kmip"
	vaultEncryption        = "vault"
	localKeyFileEncryption = "localKeyFile"
)

type diagnosticDataCollector struct {
	ctx  context.Context
	base *baseCollector

	compatibleMode bool
	topologyInfo   labelsGetter
}

// newDiagnosticDataCollector creates a collector for diagnostic information.
func newDiagnosticDataCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, compatible bool, topology labelsGetter) *diagnosticDataCollector {
	return &diagnosticDataCollector{
		ctx:  ctx,
		base: newBaseCollector(client, logger),

		compatibleMode: compatible,
		topologyInfo:   topology,
	}
}

func (d *diagnosticDataCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(d.ctx, ch, d.collect)
}

func (d *diagnosticDataCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *diagnosticDataCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "diagnostic_data")()

	var m bson.M

	logger := d.base.logger
	client := d.base.client

	cmd := bson.D{{Key: "getDiagnosticData", Value: "1"}}
	res := client.Database("admin").RunCommand(d.ctx, cmd)
	if res.Err() != nil {
		if isArbiter, _ := isArbiter(d.ctx, client); isArbiter {
			return
		}
	}

	if err := res.Decode(&m); err != nil {
		logger.Errorf("cannot run getDiagnosticData: %s", err)
	}

	if m == nil || m["data"] == nil {
		logger.Error("cannot run getDiagnosticData: response is empty")
	}

	m, ok := m["data"].(bson.M)
	if !ok {
		err := errors.Wrapf(errUnexpectedDataType, "%T for data field", m["data"])
		logger.Errorf("cannot decode getDiagnosticData: %s", err)
	}

	logger.Debug("getDiagnosticData result")
	debugResult(logger, m)

	metrics := makeMetrics("", m, d.topologyInfo.baseLabels(), d.compatibleMode)
	metrics = append(metrics, locksMetrics(logger, m)...)

	securityMetric, err := d.getSecurityMetricFromLineOptions(client)
	if err != nil {
		logger.Errorf("cannot decode getCmdLineOtpions: %s", err)
	} else if securityMetric != nil {
		metrics = append(metrics, securityMetric)
	}

	if d.compatibleMode {
		metrics = append(metrics, specialMetrics(d.ctx, client, m, logger)...)

		if cem, err := cacheEvictedTotalMetric(m); err == nil {
			metrics = append(metrics, cem)
		}

		nodeType, err := getNodeType(d.ctx, client)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"component": "diagnosticDataCollector",
			}).Errorf("Cannot get node type to check if this is a mongos: %s", err)
		} else if nodeType == typeMongos {
			metrics = append(metrics, mongosMetrics(d.ctx, client, logger)...)
		}
	}

	for _, metric := range metrics {
		ch <- metric
	}
}

func (d *diagnosticDataCollector) getSecurityMetricFromLineOptions(client *mongo.Client) (prometheus.Metric, error) {
	var cmdLineOpionsBson bson.M
	cmdLineOptions := bson.D{{Key: "getCmdLineOpts", Value: "1"}}
	resCmdLineOptions := client.Database("admin").RunCommand(d.ctx, cmdLineOptions)
	if resCmdLineOptions.Err() != nil {
		return nil, errors.Wrap(resCmdLineOptions.Err(), "cannot execute getCmdLineOpts command")
	}
	if err := resCmdLineOptions.Decode(&cmdLineOpionsBson); err != nil {
		return nil, errors.Wrap(err, "cannot parse response of the getCmdLineOpts command")
	}

	if cmdLineOpionsBson == nil || cmdLineOpionsBson["parsed"] == nil {
		return nil, errors.New("cmdlined options is empty")
	}
	parsedOptions, ok := cmdLineOpionsBson["parsed"].(bson.M)
	if !ok {
		return nil, errors.New("cannot cast parsed options to BSON")
	}
	securityOptions, ok := parsedOptions["security"].(bson.M)
	if !ok {
		return nil, nil
	}

	metric, err := d.retrieveSecurityEncryptionMetric(securityOptions)
	if err != nil {
		return nil, err
	}

	return metric, nil
}

func (d *diagnosticDataCollector) retrieveSecurityEncryptionMetric(securityOptions bson.M) (prometheus.Metric, error) {
	_, ok := securityOptions["enableEncryption"]
	if !ok {
		return nil, nil
	}

	var encryptionType string
	_, ok = securityOptions["kmip"]
	if ok {
		encryptionType = kmipEncryption
	}
	_, ok = securityOptions["vault"]
	if ok {
		encryptionType = vaultEncryption
	}
	_, ok = securityOptions["encryptionKeyFile"]
	if ok {
		encryptionType = localKeyFileEncryption
	}

	labels := map[string]string{"type": encryptionType}
	desc := prometheus.NewDesc("mongodb_security_encryption_enabled", "Shows that encryption is enabled",
		nil, labels)
	metric, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, float64(1))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create metric mongodb_security_encryption_enabled")
	}

	return metric, nil
}

// check interface.
var _ prometheus.Collector = (*diagnosticDataCollector)(nil)
