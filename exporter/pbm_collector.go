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

	// Log what we're creating
	fullName := prefix + name

	// Create the descriptor with labels as constant labels (4th parameter)
	d := prometheus.NewDesc(fullName, help, nil, labels)

	// Create the metric
	metric := prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value)

	return metric
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

func (p *pbmCollector) pbmAgentMetrics(ctx context.Context, pbmClient *sdk.Client, l *slog.Logger) []prometheus.Metric {
	currentNode, err := util.MyRole(ctx, p.base.client)
	if err != nil {
		l.Error("failed to get current node info", "error", err.Error())
		return nil
	}

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

func (p *pbmCollector) pbmBackupsMetrics(ctx context.Context, pbmClient *sdk.Client, l *slog.Logger) []prometheus.Metric {
	l.Info("===== Starting PBM backups metrics collection =====")

	currentNode, err := util.MyRole(ctx, p.base.client)
	if err != nil {
		l.Error("failed to get current node info", "error", err.Error())
		return nil
	}
	l.Info("Current node info", "me", currentNode.Me, "is_master", currentNode.IsMaster, "is_secondary", currentNode.Secondary, "primary", currentNode.Primary)

	backupsList, err := pbmClient.GetAllBackups(ctx)
	if err != nil {
		l.Error("failed to get PBM backup list", "error", err.Error())
		return nil
	}
	l.Info("Retrieved backups list", "total_backups", len(backupsList))

	metrics := make([]prometheus.Metric, 0, len(backupsList))

	for idx, backup := range backupsList {
		l.Info("===== Processing backup =====",
			"backup_index", idx,
			"name", backup.Name,
			"opid", backup.OPID,
			"status", backup.Status,
			"size", backup.Size,
			"replsets_count", len(backup.Replsets))

		// Iterate through replsets in the backup metadata
		for i, replset := range backup.Replsets {
			l.Info("--- Processing replset ---",
				"replset_index", i,
				"replset_name", replset.Name,
				"replset_node", replset.Node)

			// Determine if this is the current node
			self := "0"
			if replset.Node == currentNode.Me {
				l.Info("This replset node matches current node", "node", replset.Node)
				self = "1"
			} else {
				l.Info("This replset node does NOT match current node",
					"replset_node", replset.Node,
					"current_node", currentNode.Me)
			}

			labels := map[string]string{
				"opid":        backup.OPID,
				"status":      string(backup.Status),
				"name":        backup.Name,
				"host":        replset.Node,
				"replica_set": replset.Name,
				"self":        self,
			}

			l.Info("Created labels map for backup metrics")
			l.Info("  Label: opid", "value", labels["opid"])
			l.Info("  Label: status", "value", labels["status"])
			l.Info("  Label: name", "value", labels["name"])
			l.Info("  Label: host", "value", labels["host"])
			l.Info("  Label: replica_set", "value", labels["replica_set"])
			l.Info("  Label: self", "value", labels["self"])

			l.Info("Creating backup_size_bytes metric", "size", backup.Size)
			metric1 := createPBMMetric("backup_size_bytes",
				"Size of PBM backup",
				float64(backup.Size), labels)
			metrics = append(metrics, metric1)
			l.Info("Added backup_size_bytes metric", "total_metrics", len(metrics))

			// Add backup_last_transition_ts metric
			l.Info("Creating backup_last_transition_ts metric", "timestamp", backup.LastTransitionTS)
			metric2 := createPBMMetric("backup_last_transition_ts",
				"Last transition timestamp of PBM backup (seconds since epoch)",
				float64(backup.LastTransitionTS), labels)
			metrics = append(metrics, metric2)
			l.Info("Added backup_last_transition_ts metric", "total_metrics", len(metrics))

			var endTime int64
			switch pbmAgentStatus(backup.Status) {
			case statusDone, statusCancelled, statusError, statusDown:
				endTime = backup.LastTransitionTS
			default:
				endTime = time.Now().Unix()
			}

			duration := time.Unix(endTime-backup.StartTS, 0).Unix()
			l.Info("Creating backup_duration_seconds metric", "duration", duration, "start_ts", backup.StartTS, "end_time", endTime)
			metric3 := createPBMMetric("backup_duration_seconds",
				"Duration of PBM backup",
				float64(duration), labels)
			metrics = append(metrics, metric3)
			l.Info("Added backup_duration_seconds metric", "total_metrics", len(metrics))
		}
	}

	l.Info("===== Finished PBM backups metrics collection =====", "total_metrics_created", len(metrics))
	return metrics
}

var _ prometheus.Collector = (*pbmCollector)(nil)
