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

type pbmCollector struct {
	ctx  context.Context
	base *baseCollector

	topologyInfo        labelsGetter
	limitBackupRestores int64
}

const (
	statusDone  = "done"
	statusError = "error"

	adminDB = "admin"

	kb = 1024
	mb = 1024 * kb
	gb = 1024 * mb
	tb = 1024 * gb
	pt = 1024 * tb

	base10 = 10
)

var (
	descriptionPBMRestoreError = prometheus.NewDesc("mongodb_pbm_restore_error", "Info about failed PBM restores of backup", []string{ //nolint:gochecknoglobals
		"start_time", "last_transaction_ts", "nss", "error", "mongodb_version", "pbm_version",
	}, nil)
	descriptionPBMRestoreSuccess = prometheus.NewDesc("mongodb_pbm_restore_success", "Info about successfully PBM restores of backup", []string{ //nolint:gochecknoglobals
		"start_time", "last_transaction_ts", "nss", "status", "mongodb_version", "pbm_version",
	}, nil)
	descriptionPBMRestoreUnfinished = prometheus.NewDesc("mongodb_pbm_restore_unfinished", "Info about unfinished PBM restores of backup", []string{ //nolint:gochecknoglobals
		"start_time", "last_transaction_ts", "nss", "status", "mongodb_version", "pbm_version",
	}, nil)

	descriptionPBMBackupError = prometheus.NewDesc("mongodb_pbm_backup_error", "Info about failed PBM backups", []string{ //nolint:gochecknoglobals
		"start_time", "end_time", "mongodb_version", "pbm_version", "storage", "nss", "error",
	}, nil)
	descriptionPBMBackupSuccess = prometheus.NewDesc("mongodb_pbm_backup_success", "Info about successfully PBM backups", []string{ //nolint:gochecknoglobals
		"start_time", "end_time", "mongodb_version", "pbm_version", "storage", "nss", "status",
	}, nil)
	descriptionPBMBackupUnfinished = prometheus.NewDesc("mongodb_pbm_backup_unfinished", "Info about unfinished PBM backups", []string{ //nolint:gochecknoglobals
		"start_time", "mongodb_version", "pbm_version", "storage", "nss", "status",
	}, nil)

	descriptionPBMBackupTotal  = prometheus.NewDesc("mongodb_pbm_backup_total", "Info about all PBM backups", nil, nil)   //nolint:gochecknoglobals
	descriptionPBMRestoreTotal = prometheus.NewDesc("mongodb_pbm_restore_total", "Info about all PBM restores", nil, nil) //nolint:gochecknoglobals

	sizeBuckets = []float64{ //nolint:gochecknoglobals
		0, kb, 2 * kb, 10 * kb, 20 * kb, 100 * kb, mb, 10 * mb, 20 * mb, 50 * mb, 100 * mb,
		200 * mb, 500 * mb, gb, 2 * gb, 5 * gb, 10 * gb, 100 * gb, 200 * gb, 500 * gb, tb,
		2 * tb, 5 * tb, 10 * tb, 100 * tb, pt, 10 * pt, 100 * pt,
	}

	pbmBackupSizeMetricOpts = prometheus.HistogramOpts{ //nolint:exhaustruct,gochecknoglobals
		Name:    "mongodb_pbm_backup_size",
		Help:    "Size of the PBM backups",
		Buckets: sizeBuckets,
	}
	pbmRestoreSizeMetricOpts = prometheus.HistogramOpts{ //nolint:exhaustruct,gochecknoglobals
		Name:    "mongodb_pbm_restore_size",
		Help:    "Size of the PBM restores",
		Buckets: sizeBuckets,
	}

	speedBuckets = []float64{ //nolint:gochecknoglobals
		0, 100, kb, 2 * kb, 5 * kb, 10 * kb, 20 * kb, 50 * kb, 100 * kb, 200 * kb,
		500 * kb, mb, 10 * mb, 20 * mb, 50 * mb, 100 * mb, 150 * mb, 200 * mb,
		300 * mb, 500 * mb, 10 * gb,
	}

	pbmBackupSpeedMetricOpts = prometheus.HistogramOpts{ //nolint:exhaustruct
		Name:    "mongodb_pbm_backup_speed",
		Help:    "Speed of the creating PBM backups (only successful backups counted) in bytes per second",
		Buckets: speedBuckets,
	}
	pbmRestoreSpeedMetricOpts = prometheus.HistogramOpts{ //nolint:exhaustruct
		Name:    "mongodb_pbm_restore_speed",
		Help:    "Speed of the creating PBM Restores (only successful backups counted) in bytes per second",
		Buckets: speedBuckets,
	}
)

func newPbmCollector(ctx context.Context, client *mongo.Client, logger *logrus.Logger, topology labelsGetter, limitBackupRestores int64) *pbmCollector {
	return &pbmCollector{
		ctx:                 ctx,
		base:                newBaseCollector(client, logger),
		topologyInfo:        topology,
		limitBackupRestores: limitBackupRestores,
	}
}

func isPbmConfigured(ctx context.Context, client *mongo.Client) (bool, error) {
	names, err := client.Database(adminDB).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return false, errors.Wrap(err, "cannot get the collection list names from admin")
	}
	for _, name := range names {
		if name == "pbmConfig" {
			return true, nil
		}
	}
	return false, nil
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
	StartTime          int64     `bson:"start_ts"`
	LastTransitionTime int64     `bson:"last_transition_ts"`
	Size               int64     `bson:"size"`
	Error              string    `bson:"error,omitempty"`
	Status             string    `bson:"status"`
	Type               string    `bson:"type"`
	MongoDBVersion     string    `bson:"mongodb_version"`
	PbmDBVersion       string    `bson:"pbm_version"`
	Nss                []string  `bson:"nss,omitempty"`
	Store              storeType `bson:"store"`
}

type pbmRestoreResult struct {
	StartTime          int64     `bson:"start_ts"`
	Name               string    `bson:"name"`
	LastTransitionTime int64     `bson:"last_transition_ts"`
	Status             string    `bson:"status"`
	Error              string    `bson:"error,omitempty"`
	Type               string    `bson:"type"`
	Nss                []string  `bson:"nss,omitempty"`
	Size               int64     `bson:"size"`
	MongoDBVersion     string    `bson:"mongodb_version"`
	PbmVersion         string    `bson:"pbm_version"`
	Store              storeType `bson:"store"`
}

func (p *pbmCollector) collect(ch chan<- prometheus.Metric) {
	defer prometheus.MeasureCollectTime(ch, "mongodb", "pbm_stats")()

	logger := p.base.logger
	logger.Debug("collect PBM stats")

	if err := p.collectPbmBackupMetrics(ch); err != nil {
		logger.Errorf("cannot create PBM Backup metrics: %s", err)
		return
	}

	if err := p.collectPbmRestoreMetrics(ch); err != nil {
		logger.Errorf("cannot create PBM Restore metrics: %s", err)
		return
	}
}

func (p *pbmCollector) collectPbmBackupMetrics(ch chan<- prometheus.Metric) error { //nolint:funlen,cyclop
	pbmBackupResults, err := p.retrievePbmBackupInfo()
	if err != nil {
		return err
	}
	var metrics []prometheus.Metric
	pbmBackupSizeMetric := prometheus.NewHistogramVec(pbmBackupSizeMetricOpts, []string{"storage"})
	pbmBackupSpeedMetric := prometheus.NewHistogramVec(pbmBackupSpeedMetricOpts, []string{"storage"})

	for _, result := range pbmBackupResults {
		nss := ""
		if len(result.Nss) > 0 {
			nss = result.Nss[0]
		}

		startTimeUnix := strconv.FormatInt(result.StartTime, base10)
		endTimeUnix := strconv.FormatInt(result.LastTransitionTime, base10)

		switch result.Status {
		case statusError:
			metric, err := prometheus.NewConstMetric(descriptionPBMBackupError, prometheus.GaugeValue, 1, []string{
				startTimeUnix, endTimeUnix, result.MongoDBVersion, result.PbmDBVersion, result.Store.Type, nss, result.Error,
			}...)
			if err != nil {
				p.base.logger.Error("Cannot create metrics 'mongodb_pbm_backup_error'", err)
				return err
			}
			metrics = append(metrics, metric)
		case statusDone:
			metric, err := prometheus.NewConstMetric(descriptionPBMBackupSuccess, prometheus.GaugeValue, 1, []string{
				startTimeUnix, endTimeUnix, result.MongoDBVersion, result.PbmDBVersion, result.Store.Type, nss, result.Status,
			}...)
			if err != nil {
				p.base.logger.Error("Cannot create metrics 'mongodb_pbm_backup_success'", err)
				return err
			}
			metrics = append(metrics, metric)

			// set size only for successfully backups
			pbmBackupSizeMetric.WithLabelValues(result.Store.Type).Observe(float64(result.Size))
			pbmBackupSpeedMetric.WithLabelValues(result.Store.Type).Observe(float64(result.Size) / float64(result.LastTransitionTime-result.StartTime))
		default:
			metric, err := prometheus.NewConstMetric(descriptionPBMBackupUnfinished, prometheus.GaugeValue, 1, []string{
				startTimeUnix, result.MongoDBVersion, result.PbmDBVersion, result.Store.Type, nss, result.Status,
			}...)
			if err != nil {
				p.base.logger.Error("Cannot create metrics 'mongodb_pbm_backup_unfinished'", err)
				return err
			}
			metrics = append(metrics, metric)
		}
	}

	totalBackupMetric, err := prometheus.NewConstMetric(descriptionPBMBackupTotal, prometheus.GaugeValue, float64(len(pbmBackupResults)))
	if err != nil {
		p.base.logger.Error("Cannot create metrics 'mongodb_pbm_backup_total'", err)
		return err
	}
	metrics = append(metrics, totalBackupMetric)

	for _, metric := range metrics {
		ch <- metric
	}

	pbmBackupSizeMetric.Collect(ch)
	pbmBackupSpeedMetric.Collect(ch)

	return nil
}

func (p *pbmCollector) retrievePbmBackupInfo() ([]pbmBackupResult, error) {
	client := p.base.client

	pbmBackupCollection := client.Database(adminDB).Collection("pbmBackups")
	opts := options.Find().SetSort(bson.D{primitive.E{"hb", -1}}).SetLimit(p.limitBackupRestores)
	pbmBackupsRes, err := pbmBackupCollection.Find(p.ctx, bson.D{}, opts)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve cursor from 'pbmBackups'")
	}
	defer pbmBackupsRes.Close(p.ctx) //nolint:errcheck

	var pbmBackupResults []pbmBackupResult
	if err := pbmBackupsRes.All(p.ctx, &pbmBackupResults); err != nil {
		return nil, errors.Wrap(err, "cannot parse query result to objects")
	}
	return pbmBackupResults, nil
}

func (p *pbmCollector) collectPbmRestoreMetrics(ch chan<- prometheus.Metric) error { //nolint:funlen,cyclop
	pbmRestoreResults, err := p.retrievePbmRestoreInfo()
	if err != nil {
		return err
	}
	pbmRestoreSizeMetric := prometheus.NewHistogramVec(pbmRestoreSizeMetricOpts, []string{"storage"})
	pbmRestoreSpeedMetric := prometheus.NewHistogramVec(pbmRestoreSpeedMetricOpts, []string{"storage"})

	var metrics []prometheus.Metric
	for _, result := range pbmRestoreResults {
		nss := ""
		if len(result.Nss) > 0 {
			nss = result.Nss[0]
		}

		startTimeUnix := strconv.FormatInt(time.Unix(result.StartTime, 0).Unix(), base10)
		lastTransactionTS := strconv.FormatInt(time.Unix(result.LastTransitionTime, 0).Unix(), base10)

		switch result.Status {
		case statusError:
			metric, err := prometheus.NewConstMetric(descriptionPBMRestoreError, prometheus.GaugeValue, 1, []string{
				startTimeUnix, lastTransactionTS, nss, result.Error, result.MongoDBVersion, result.PbmVersion,
			}...)
			if err != nil {
				p.base.logger.Error("Cannot create metrics 'mongodb_pbm_restore_error'", err)
				return err
			}
			metrics = append(metrics, metric)
		case statusDone:
			metric, err := prometheus.NewConstMetric(descriptionPBMRestoreSuccess, prometheus.GaugeValue, 1, []string{
				startTimeUnix, lastTransactionTS, nss, result.Status, result.MongoDBVersion, result.PbmVersion,
			}...)
			if err != nil {
				p.base.logger.Error("Cannot create metrics 'mongodb_pbm_restore_success'", err)
				return err
			}
			metrics = append(metrics, metric)

			// set size only for successfully restores
			pbmRestoreSizeMetric.WithLabelValues(result.Store.Type).Observe(float64(result.Size))
			pbmRestoreSpeedMetric.WithLabelValues(result.Store.Type).Observe(float64(result.Size) / float64(result.LastTransitionTime-result.StartTime))
		default:
			metric, err := prometheus.NewConstMetric(descriptionPBMRestoreUnfinished, prometheus.GaugeValue, 1, []string{
				startTimeUnix, lastTransactionTS, nss, result.Status, result.MongoDBVersion, result.PbmVersion,
			}...)
			if err != nil {
				p.base.logger.Error("Cannot create metrics 'mongodb_pbm_restore_unfinished'", err)
				return err
			}
			metrics = append(metrics, metric)
		}
	}

	totalBackupMetric, err := prometheus.NewConstMetric(descriptionPBMRestoreTotal, prometheus.GaugeValue, float64(len(pbmRestoreResults)))
	if err != nil {
		p.base.logger.Error("Cannot create metrics 'mongodb_pbm_backup_total'", err)
		return err
	}
	metrics = append(metrics, totalBackupMetric)

	for _, metric := range metrics {
		ch <- metric
	}

	pbmRestoreSizeMetric.Collect(ch)
	pbmRestoreSpeedMetric.Collect(ch)

	return nil
}

func (p *pbmCollector) retrievePbmRestoreInfo() ([]pbmRestoreResult, error) {
	client := p.base.client

	pbmBackupCollection := client.Database(adminDB).Collection("pbmRestores")

	sortStage := bson.D{primitive.E{Key: "$sort", Value: bson.D{primitive.E{"last_transition_ts", -1}}}}
	limitStage := bson.D{primitive.E{Key: "$limit", Value: p.limitBackupRestores}}
	lookupStage := bson.D{primitive.E{Key: "$lookup", Value: bson.D{
		{"from", "pbmBackups"},
		{"localField", "backup"},
		{"foreignField", "name"},
		{"as", "pbmBackups"},
	}}}
	projectStage := bson.D{primitive.E{Key: "$project",
		Value: bson.D{
			{"status", 1},
			{"name", 1},
			{"last_transaction_ts", 1},
			{"backup", 1},
			{"start_ts", 1},
			{"last_transition_ts", 1},
			{"type", 1},
			{"error", 1},
			{"nss", 1},
			{"res", 1},
			{"size", bson.D{{"$first", "$pbmBackups.size"}}},
			{"store", bson.D{{"$first", "$pbmBackups.store"}}},
			{"mongodb_version", bson.D{{"$first", "$pbmBackups.mongodb_version"}}},
			{"pbm_version", bson.D{{"$first", "$pbmBackups.pbm_version"}}},
		}}}

	pbmRestoresRes, err := pbmBackupCollection.Aggregate(p.ctx, mongo.Pipeline{lookupStage, limitStage, sortStage, projectStage})
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve cursor from 'pbmRestores'")
	}
	defer pbmRestoresRes.Close(p.ctx) //nolint:errcheck

	var pbmBackupResults []pbmRestoreResult
	if err := pbmRestoresRes.All(p.ctx, &pbmBackupResults); err != nil {
		return nil, errors.Wrap(err, "cannot parse query result to objects")
	}
	return pbmBackupResults, nil
}
