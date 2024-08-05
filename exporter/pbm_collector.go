// mongodb_exporter
// Copyright (C) 2024 Percona LLC
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

	"github.com/percona/percona-backup-mongodb/sdk"
	"github.com/percona/percona-backup-mongodb/sdk/cli"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

// pbm collector collects metrics from PBM (Percona Backup for MongoDb).
type pbmCollector struct {
	ctx      context.Context
	mongoURI string
	base     *baseCollector
}

const (
	pbmAgentStatusOK    = 0
	pbmAgentStatusError = 1
	pbmAgentStatusLost  = 2

	statusDown      = "down"
	statusDone      = "done"
	statusCancelled = "canceled"
	statusError     = "error"
)

func createMetric(name, help string, value float64, labels map[string]string) prometheus.Metric {
	const prefix = "mongodb_pbm_"
	d := prometheus.NewDesc(prefix+name, help, nil, labels)
	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value)
}

func newPbmCollector(ctx context.Context, client *mongo.Client, mongoURI string, logger *logrus.Logger) *pbmCollector {
	return &pbmCollector{
		ctx:      ctx,
		mongoURI: mongoURI,
		base:     newBaseCollector(client, logger.WithFields(logrus.Fields{"collector": "pbm"})),
	}
}

func (p *pbmCollector) Describe(ch chan<- *prometheus.Desc) {
	p.base.Describe(p.ctx, ch, p.collect)
}

func (p *pbmCollector) Collect(ch chan<- prometheus.Metric) {
	p.base.Collect(ch)
}

func (p *pbmCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "pbm")()

	var metrics []prometheus.Metric
	logger := p.base.logger

	pbmClient, err := sdk.NewClient(p.ctx, p.mongoURI)
	if err != nil {
		logger.Errorf("failed to create PBM client: %s", err.Error())
		return
	}

	pbmConfig, err := pbmClient.GetConfig(p.ctx)
	if err != nil {
		logger.Errorf("failed to get PBM configuration: %s", err.Error())
		return
	}

	if pbmConfig != nil {
		metrics = append(metrics, createMetric("cluster_backup_configured",
			"PBM backups are configured for the cluster",
			float64(1), nil))

		pitrEnabledMetric := float64(0)
		if pbmConfig.PITR.Enabled {
			pitrEnabledMetric = 1
		}

		metrics = append(metrics, createMetric("cluster_pitr_backup_enabled",
			"PBM PITR backups are enabled for the cluster",
			pitrEnabledMetric, nil))
	}

	metrics = append(metrics, p.pbmBackupsMetrics(p.ctx, pbmClient, logger)...)
	metrics = append(metrics, p.pbmAgentMetrics(p.ctx, pbmClient, logger)...)

	for _, metric := range metrics {
		ch <- metric
	}
}

func (p *pbmCollector) pbmAgentMetrics(ctx context.Context, pbmClient *sdk.Client, l *logrus.Entry) []prometheus.Metric {
	clusterStatus, err := cli.ClusterStatus(ctx, pbmClient, cli.RSConfGetter(p.mongoURI))
	if err != nil {
		l.Errorf("failed to get cluster status: %s", err.Error())
		return nil
	}

	metrics := make([]prometheus.Metric, 0, len(clusterStatus))

	for replsetName, nodes := range clusterStatus {
		for _, node := range nodes {
			pbmStatusMetric := float64(1)
			switch {
			case node.OK:
				pbmStatusMetric = float64(pbmAgentStatusOK)

			case node.IsAgentLost():
				pbmStatusMetric = float64(pbmAgentStatusLost)

			default: // !node.OK
				pbmStatusMetric = float64(pbmAgentStatusError)
			}
			metrics = append(metrics, createMetric("agent_status",
				"PBM Agent Status",
				pbmStatusMetric,
				map[string]string{
					"host":        node.Host,
					"replica_set": replsetName,
					"role":        string(node.Role),
				}),
			)
		}
	}

	return metrics
}

func (p *pbmCollector) pbmBackupsMetrics(ctx context.Context, pbmClient *sdk.Client, l *logrus.Entry) []prometheus.Metric {
	backupsList, err := pbmClient.GetAllBackups(ctx)
	if err != nil {
		l.Errorf("failed to get PBM backup list: %s", err.Error())
		return nil
	}

	metrics := make([]prometheus.Metric, 0, len(backupsList))

	for _, backup := range backupsList {
		metrics = append(metrics, createMetric("backup_size",
			"Size of PBM backup",
			float64(backup.Size), map[string]string{
				"opid":   backup.OPID,
				"status": string(backup.Status),
				"name":   backup.Name,
			}),
		)

		var endTime int64
		switch string(backup.Status) { //nolint:exhaustive
		case statusDone, statusCancelled, statusError, statusDown:
			endTime = backup.LastTransitionTS
		default:
			endTime = time.Now().Unix()
		}

		duration := time.Unix(endTime-backup.StartTS, 0).Unix()
		metrics = append(metrics, createMetric("backup_duration_seconds",
			"Duration of PBM backup",
			float64(duration), map[string]string{
				"opid":   backup.OPID,
				"status": string(backup.Status),
				"name":   backup.Name,
			}),
		)
	}
	return metrics
}

var _ prometheus.Collector = (*pbmCollector)(nil)
