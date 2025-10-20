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
	"log/slog"
	"strings"
	"time"

	"github.com/percona/percona-backup-mongodb/sdk"
	"github.com/percona/percona-backup-mongodb/sdk/cli"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/internal/proto"
	"github.com/percona/mongodb_exporter/internal/util"
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

func newPbmCollector(ctx context.Context, client *mongo.Client, mongoURI string, logger *slog.Logger) *pbmCollector {
	// we can't get details of other cluster members from PBM if directConnection is set to true,
	// we re-write it if that option is set (e.g from PMM).
	if strings.Contains(mongoURI, "directConnection=true") {
		logger.Debug("directConnection is enabled, will be disabled for PBM collector.")
		mongoURI = strings.ReplaceAll(mongoURI, "directConnection=true", "directConnection=false")
	}
	return &pbmCollector{
		ctx:      ctx,
		mongoURI: mongoURI,
		base:     newBaseCollector(client, logger.With("collector", "pbm")),
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

	pbmEnabledMetric := 0
	pbmClient, err := sdk.NewClient(p.ctx, p.mongoURI)
	if err != nil {
		logger.Warn("failed to create PBM client", "error", err.Error())
		return
	}
	defer func() {
		err := pbmClient.Close(p.ctx)
		if err != nil {
			logger.Error("failed to close PBM client", "error", err)
		}
	}()

	pbmConfig, err := pbmClient.GetConfig(p.ctx)
	if err != nil {
		logger.Info("failed to get PBM configuration", "error", err.Error())
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

		// Get current node info once for both agent and backup metrics
		currentNode, err := util.MyRole(p.ctx, p.base.client)
		if err != nil {
			logger.Error("failed to get current node info", "error", err.Error())
		} else {
			metrics = append(metrics, p.pbmBackupsMetrics(p.ctx, pbmClient, logger, currentNode)...)
			metrics = append(metrics, p.pbmAgentMetrics(p.ctx, pbmClient, logger, currentNode)...)
		}
	}

	metrics = append(metrics, createPBMMetric("cluster_backup_configured",
		"PBM backups are configured for the cluster",
		float64(pbmEnabledMetric), nil))

	for _, metric := range metrics {
		ch <- metric
	}
}

func (p *pbmCollector) pbmAgentMetrics(ctx context.Context, pbmClient *sdk.Client, l *slog.Logger, currentNode *proto.HelloResponse) []prometheus.Metric {
	clusterStatus, err := cli.ClusterStatus(ctx, pbmClient, cli.RSConfGetter(p.mongoURI))
	if err != nil {
		l.Error("failed to get cluster status", "error", err.Error())
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
			labels := map[string]string{
				"host":        node.Host,
				"replica_set": replsetName,
				"role":        string(node.Role),
				"self":        "0",
			}
			if node.Host == currentNode.Me {
				labels["self"] = "1"
			}
			metrics = append(metrics, createPBMMetric("agent_status",
				"PBM Agent Status",
				pbmStatusMetric,
				labels,
			),
			)
		}
	}

	return metrics
}

func (p *pbmCollector) pbmBackupsMetrics(ctx context.Context, pbmClient *sdk.Client, l *slog.Logger, currentNode *proto.HelloResponse) []prometheus.Metric {
	backupsList, err := pbmClient.GetAllBackups(ctx)
	if err != nil {
		l.Error("failed to get PBM backup list", "error", err.Error())
		return nil
	}

	metrics := make([]prometheus.Metric, 0, len(backupsList))

	for _, backup := range backupsList {
		// Iterate through replsets in the backup metadata
		for _, replset := range backup.Replsets {
			// Determine if this is the current node
			self := "0"
			if replset.Node == currentNode.Me {
				self = "1"
			}

			labels := map[string]string{
				"opid":        backup.OPID,
				"status":      string(backup.Status),
				"name":        backup.Name,
				"host":        replset.Node,
				"replica_set": replset.Name,
				"self":        self,
				"type":        string(backup.Type),
			}

			metrics = append(metrics, createPBMMetric("backup_size_bytes",
				"Size of PBM backup",
				float64(backup.Size), labels),
			)

			// Add backup_last_transition_ts metric
			metrics = append(metrics, createPBMMetric("backup_last_transition_ts",
				"Last transition timestamp of PBM backup (seconds since epoch)",
				float64(backup.LastTransitionTS), labels),
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
				float64(duration), labels),
			)
		}
	}
	return metrics
}

var _ prometheus.Collector = (*pbmCollector)(nil)
