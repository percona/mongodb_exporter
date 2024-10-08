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
	"strings"
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

type pbmAgentStatus string

const (
	pbmAgentStatusOK    = iota
	pbmAgentStatusError = iota
	pbmAgentStatusLost  = iota

	statusDown      pbmAgentStatus = "down"
	statusDone      pbmAgentStatus = "done"
	statusCancelled pbmAgentStatus = "canceled"
	statusError     pbmAgentStatus = "error"
)

func createPBMMetric(name, help string, value float64, labels map[string]string) prometheus.Metric { //nolint:ireturn
	const prefix = "mongodb_pbm_"
	d := prometheus.NewDesc(prefix+name, help, nil, labels)
	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value)
}

func newPbmCollector(ctx context.Context, client *mongo.Client, mongoURI string, logger *logrus.Logger) (*pbmCollector, error) {
	// we can't get details of other cluster from PBM if directConnection is set to true,
	// we re-write it if that option is set (e.g from PMM).
	if strings.Contains(mongoURI, "directConnection=true") {
		mongoURI = strings.ReplaceAll(mongoURI, "directConnection=true", "directConnection=false")
	}
	pbmClient, err := sdk.NewClient(ctx, mongoURI)
	if err != nil {
		logger.Errorf("failed to initialize PBM client from uri %s: %s", mongoURI, err.Error())
		return nil, err
	}

	defer func() {
		err := pbmClient.Close(ctx)
		if err != nil {
			logger.Errorf("failed to close PBM client: %v", err)
		}
	}()
	_, err = pbmClient.GetConfig(ctx)
	if err != nil {
		logger.Errorf("failed to get PBM configuration during initialization: %s", err.Error())
		return nil, err
	}
	return &pbmCollector{
		ctx:      ctx,
		mongoURI: mongoURI,
		base:     newBaseCollector(client, logger.WithFields(logrus.Fields{"collector": "pbm"})),
	}, nil
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

	pbmEnabledMetric := 0
	pbmClient, err := sdk.NewClient(p.ctx, p.mongoURI)
	if err != nil {
		logger.Errorf("failed to create PBM client from uri %s: %s", p.mongoURI, err.Error())
		return
	}
	defer func() {
		err := pbmClient.Close(p.ctx)
		if err != nil {
			logger.Errorf("failed to close PBM client: %v", err)
		}
	}()

	pbmConfig, err := pbmClient.GetConfig(p.ctx)
	if err != nil {
		logger.Errorf("failed to get PBM configuration: %s", err.Error())
	}

	if pbmConfig != nil {
		pbmEnabledMetric = 1

		pitrEnabledMetric := 0
		if pbmConfig.PITR.Enabled {
			pitrEnabledMetric = 1
		}

		metrics = append(metrics, createPBMMetric("cluster_pitr_backup_enabled",
			"PBM PITR backups are enabled for the cluster",
			float64(pitrEnabledMetric), nil))

		metrics = append(metrics, p.pbmBackupsMetrics(p.ctx, pbmClient, logger)...)
		metrics = append(metrics, p.pbmAgentMetrics(p.ctx, pbmClient, logger)...)
	}

	metrics = append(metrics, createPBMMetric("cluster_backup_configured",
		"PBM backups are configured for the cluster",
		float64(pbmEnabledMetric), nil))

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
			var pbmStatusMetric float64
			switch {
			case node.OK:
				pbmStatusMetric = float64(pbmAgentStatusOK)

			case node.IsAgentLost():
				pbmStatusMetric = float64(pbmAgentStatusLost)

			default: // !node.OK
				pbmStatusMetric = float64(pbmAgentStatusError)
			}
			metrics = append(metrics, createPBMMetric("agent_status",
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
		metrics = append(metrics, createPBMMetric("backup_size_bytes",
			"Size of PBM backup",
			float64(backup.Size), map[string]string{
				"opid":   backup.OPID,
				"status": string(backup.Status),
				"name":   backup.Name,
			}),
		)

		var endTime int64
		switch pbmAgentStatus(backup.Status) {
		case statusDone, statusCancelled, statusError, statusDown:
			endTime = backup.LastTransitionTS
		default:
			endTime = time.Now().Unix()
		}

		duration := time.Unix(endTime-backup.StartTS, 0).Unix()
		metrics = append(metrics, createPBMMetric("backup_duration_seconds",
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
