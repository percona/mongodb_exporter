package exporter

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const adminDb = "admin"

type pbmCollector struct {
	ctx  context.Context
	base *baseCollector

	topologyInfo labelsGetter
}

func newPbmCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, topology labelsGetter) *pbmCollector {
	return &pbmCollector{
		ctx:          ctx,
		base:         newBaseCollector(client, logger),
		topologyInfo: topology,
	}
}

func (p *pbmCollector) Describe(ch chan<- *prometheus.Desc) {
	p.base.Describe(p.ctx, ch, p.collect)
}

func (p *pbmCollector) Collect(ch chan<- prometheus.Metric) {
	p.base.Collect(ch)
}

type storeType struct {
	Type string `bson:"type"`
}
type pbmBackupResult struct {
	OpID               string              `bson:"opid"`
	StartTime          primitive.Timestamp `bson:"first_write_ts"`
	EndTime            primitive.Timestamp `bson:"last_write_ts"`
	Name               string              `bson:"name"`
	LastTransitionTime int64               `bson:"last_transition_ts"`
	Size               int64               `bson:"size"`
	Error              string              `bson:"error,omitempty"`
	Status             string              `bson:"status"`
	Type               string              `bson:"type"`
	MongoDBVersion     string              `bson:"mongodb_version"`
	PBMVersion         string              `bson:"pbm_version"`
	Balancer           string              `bson:"balancer"`
	Compression        string              `bson:"compression"`
	Nss                []string            `bson:"nss,omitempty"`
	Store              storeType           `bson:"store"`
}

type pbmRestoreResult struct {
	OpID               string   `bson:"opid"`
	StartTime          int64    `bson:"start_ts"`
	Name               string   `bson:"name"`
	LastTransitionTime int64    `bson:"last_transition_ts"`
	Status             string   `bson:"status"`
	Error              string   `bson:"error,omitempty"`
	Type               string   `bson:"type"`
	Nss                []string `bson:"nss,omitempty"`
}

func (p *pbmCollector) collect(ch chan<- prometheus.Metric) {
	defer prometheus.MeasureCollectTime(ch, "mongodb", "pbm_stats")()

	logger := p.base.logger
	logger.Debug("collect PBM stats")

	if err := p.collectPbmBackupMetrics(ch); err != nil {
		logger.Errorf("cannot create PBM Backup metrics: %s", err)
	}

	if err := p.collectPbmRestoreMetrics(ch); err != nil {
		logger.Errorf("cannot creat PBM Restore metrics: %s", err)
	}

	//todo: add here metric with PBM config info 'mongod_pbm_config'
}

func (p *pbmCollector) collectPbmRestoreMetrics(ch chan<- prometheus.Metric) error {

	pbmRestoreResults, err := p.retrievePbmRestoreInfo()
	if err != nil {
		return err
	}

	for _, pbmRestore := range pbmRestoreResults {
		metric := p.createRestoreMetric(pbmRestore)
		ch <- metric
	}
	return nil
}

func (p *pbmCollector) collectPbmBackupMetrics(ch chan<- prometheus.Metric) error {
	pbmBackupResults, err := p.retrievePbmBackupInfo()
	if err != nil {
		return err
	}

	for _, pbmPackup := range pbmBackupResults {
		metric := p.createBackupMetric(pbmPackup)
		ch <- metric
	}
	return nil
}

func (p *pbmCollector) retrievePbmBackupInfo() ([]pbmBackupResult, error) {
	client := p.base.client

	pbmBackupCollection := client.Database(adminDb).Collection("pbmBackups")
	// filter in the descending order by hb (heartbeat) field
	// todo: think how many rows we can show
	opts := options.Find().SetSort(bson.D{{"hb", -1}}).SetLimit(100)
	pbmBackupsRes, err := pbmBackupCollection.Find(p.ctx, bson.D{}, opts)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve cursor from 'pbmBackups'")
	}
	defer pbmBackupsRes.Close(p.ctx)

	var pbmBackupResults []pbmBackupResult
	if err := pbmBackupsRes.All(p.ctx, &pbmBackupResults); err != nil {
		return nil, errors.Wrap(err, "cannot parse query result to objects")
	}
	return pbmBackupResults, nil
}

func (p *pbmCollector) createBackupMetric(pbmPackup pbmBackupResult) prometheus.Metric {
	nss := ""
	if len(pbmPackup.Nss) > 0 {
		nss = pbmPackup.Nss[0]
	}
	startTimeOfBackup := time.Unix(int64(pbmPackup.StartTime.T), 0)
	endTimeOfBackup := time.Unix(int64(pbmPackup.EndTime.T), 0)
	labels := map[string]string{
		"opid":               pbmPackup.OpID,
		"start_time":         strconv.FormatInt(startTimeOfBackup.Unix(), 10),
		"end_time":           strconv.FormatInt(endTimeOfBackup.Unix(), 10),
		"name":               pbmPackup.Name,
		"last_transition_ts": strconv.FormatInt(pbmPackup.LastTransitionTime, 10),
		"size":               strconv.FormatInt(pbmPackup.Size, 10),
		"error":              pbmPackup.Error,
		"status":             pbmPackup.Status,
		"type":               pbmPackup.Type,
		"mongodb_version":    pbmPackup.MongoDBVersion,
		"pbm_version":        pbmPackup.PBMVersion,
		"balancer":           pbmPackup.Balancer,
		"compression":        pbmPackup.Compression,
		"store_type":         pbmPackup.Store.Type,
		"nss":                nss,
	}

	d := prometheus.NewDesc("mongodb_pbm_backup", "info about PBM backup", nil, labels)
	metric := prometheus.NewMetricWithTimestamp(startTimeOfBackup, prometheus.MustNewConstMetric(d, prometheus.GaugeValue, 1))
	return metric
}

func (p *pbmCollector) retrievePbmRestoreInfo() ([]pbmRestoreResult, error) {
	client := p.base.client

	pbmBackupCollection := client.Database(adminDb).Collection("pbmRestores")
	// filter in the descending order by hb (heartbeat) field
	// todo: think how many rows we can show
	opts := options.Find().SetSort(bson.D{{"last_transition_ts", -1}}).SetLimit(100)
	pbmRestoresRes, err := pbmBackupCollection.Find(p.ctx, bson.D{}, opts)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve cursor from 'pbmRestores'")
	}
	defer pbmRestoresRes.Close(p.ctx)

	var pbmBackupResults []pbmRestoreResult
	if err := pbmRestoresRes.All(p.ctx, &pbmBackupResults); err != nil {
		return nil, errors.Wrap(err, "cannot parse query result to objects")
	}
	return pbmBackupResults, nil
}

func (p *pbmCollector) createRestoreMetric(pbmRestore pbmRestoreResult) prometheus.Metric {
	nss := ""
	if len(pbmRestore.Nss) > 0 {
		nss = pbmRestore.Nss[0]
	}
	startTimeOfBackup := time.Unix(pbmRestore.StartTime, 0)
	labels := map[string]string{
		"opid":               pbmRestore.OpID,
		"start_time":         strconv.FormatInt(pbmRestore.StartTime, 10),
		"last_transition_ts": strconv.FormatInt(pbmRestore.LastTransitionTime, 10),
		"name":               pbmRestore.Name,
		"error":              pbmRestore.Error,
		"status":             pbmRestore.Status,
		"type":               pbmRestore.Type,
		"nss":                nss,
	}

	d := prometheus.NewDesc("mongodb_pbm_restore", "info about PBM backup", nil, labels)
	metric := prometheus.NewMetricWithTimestamp(startTimeOfBackup, prometheus.MustNewConstMetric(d, prometheus.GaugeValue, 1))
	return metric
}
