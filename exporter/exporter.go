package exporter

import (
	"context"
	"fmt"

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	client     *mongo.Client
	collectors []prometheus.Collector
	path       string
	port       int
}

// Opts holds new exporter options.
type Opts struct {
	DSN                  string
	Log                  *logrus.Logger
	Path                 string
	Port                 int
	CollStatsCollections []string
}

// New connects to the database and returns a new Exporter instance.
func New(opts *Opts) (*Exporter, error) {
	if opts == nil {
		opts = new(Opts)
	}

	client, err := connect(context.Background(), opts.DSN)
	if err != nil {
		return nil, err
	}

	exp := &Exporter{
		client:     client,
		collectors: make([]prometheus.Collector, 0),
		path:       opts.Path,
		port:       opts.Port,
	}

	return exp, nil
}

// Run starts the exporter.
func (e *Exporter) Run() {
	registry := prometheus.NewRegistry()

	for _, collector := range e.collectors {
		registry.MustRegister(collector)
	}

	gatherers := prometheus.Gatherers{}
	gatherers = append(gatherers, prometheus.DefaultGatherer)
	gatherers = append(gatherers, registry)

	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	handler := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
		ErrorLog:      log.NewErrorLogger(),
	})

	addr := fmt.Sprintf(":%d", e.port)
	exporter_shared.RunServer("MongoDB", addr, e.path, handler)
}

func connect(ctx context.Context, dsn string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(dsn)
	clientOpts.SetDirect(true)
	clientOpts.SetAppName("mnogo_exporter")

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	if err = client.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}

	return client, nil
}
