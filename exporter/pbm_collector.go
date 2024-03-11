package exporter

import (
	"context"
	"github.com/percona/percona-backup-mongodb/sdk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
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
		base:     newBaseCollector(client, logger),
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

	createMetric := func(name, help string, value float64, labels map[string]string) {
		const prefix = "mongodb_pbm_"
		d := prometheus.NewDesc(prefix+name, help, nil, labels)
		metrics = append(metrics, prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value))
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
	}

	if pbmConfig.PITR.Enabled {
		createMetric("cluster_pitr_backup_enabled",
			"PBM PITR backups are enabled for the cluster",
			float64(1), nil)
	}

	backupsList, err := pbmClient.GetAllBackups(p.ctx)
	if err != nil {
		logger.Errorf("failed to get PBM backup list: %s", err.Error())
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
		switch backup.Status {
		case "done", "canceled", "error", "down":
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

	for _, metric := range metrics {
		ch <- metric
	}
}

var _ prometheus.Collector = (*pbmCollector)(nil)
