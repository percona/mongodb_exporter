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

	"github.com/percona/percona-backup-mongodb/pbm/defs"
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

	createMetric := func(name, help string, value float64, labels map[string]string) {
		const prefix = "mongodb_pbm_"
		d := prometheus.NewDesc(prefix+name, help, nil, labels)
		metrics = append(metrics, prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value))
	}

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
		createMetric("cluster_backup_configured",
			"PBM backups are configured for the cluster",
			float64(1), nil)

		if pbmConfig.PITR.Enabled {
			createMetric("cluster_pitr_backup_enabled",
				"PBM PITR backups are enabled for the cluster",
				float64(1), nil)
		}
	}

	metrics = append(metrics, pbmBackupsMetrics(p.ctx, pbmClient, logger)...)
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
	createMetric := func(name, help string, value float64, labels map[string]string) {
		const prefix = "mongodb_pbm_"
		d := prometheus.NewDesc(prefix+name, help, nil, labels)
		metrics = append(metrics, prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value))
	}

	for replsetName, nodes := range clusterStatus {
		for _, node := range nodes {
			switch {
			case node.OK:
				createMetric("agent_status",
					"PBM Agent Status",
					float64(0),
					map[string]string{
						"host":        node.Host,
						"replica_set": replsetName,
						"role":        string(node.Role),
					})

			case node.IsAgentLost():
				createMetric("agent_status",
					"PBM Agent Status",
					float64(2),
					map[string]string{
						"host":        node.Host,
						"replica_set": replsetName,
						"role":        string(node.Role),
					})

			default: // !node.OK
				createMetric("agent_status",
					"PBM Agent Status",
					float64(1),
					map[string]string{
						"host":        node.Host,
						"replica_set": replsetName,
						"role":        string(node.Role),
					})
			}
		}
	}

	return metrics
}

func pbmBackupsMetrics(ctx context.Context, pbmClient *sdk.Client, l *logrus.Entry) []prometheus.Metric {
	backupsList, err := pbmClient.GetAllBackups(ctx)
	if err != nil {
		l.Errorf("failed to get PBM backup list: %s", err.Error())
		return nil
	}

	metrics := make([]prometheus.Metric, 0, len(backupsList))
	createMetric := func(name, help string, value float64, labels map[string]string) {
		const prefix = "mongodb_pbm_"
		d := prometheus.NewDesc(prefix+name, help, nil, labels)
		metrics = append(metrics, prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value))
	}

	for _, backup := range backupsList {
		createMetric("backup_size",
			"Size of PBM backup",
			float64(backup.Size), map[string]string{
				"opid":   backup.OPID,
				"status": string(backup.Status),
				"name":   backup.Name,
			})

		var endTime int64
		switch backup.Status { //nolint:exhaustive
		case defs.StatusDone, defs.StatusCancelled, defs.StatusError, defs.StatusDown:
			endTime = backup.LastTransitionTS
		default:
			endTime = time.Now().Unix()
		}

		duration := time.Unix(endTime-backup.StartTS, 0).Unix()
		createMetric("backup_duration_seconds",
			"Duration of PBM backup",
			float64(duration), map[string]string{
				"opid":   backup.OPID,
				"status": string(backup.Status),
				"name":   backup.Name,
			})
	}
	return metrics
}

var _ prometheus.Collector = (*pbmCollector)(nil)
