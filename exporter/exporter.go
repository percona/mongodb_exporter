// mnogo_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package exporter

import (
	"context"
	"fmt"

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	path       string
	port       int
	client     *mongo.Client
	log        *logrus.Logger
	collectors []prometheus.Collector
}

// Opts holds new exporter options.
type Opts struct {
	CollStatsCollections []string
	DSN                  string
	Log                  *logrus.Logger
	Path                 string
	Port                 int
}

var (
	errCannotHandleType   = fmt.Errorf("don't know how to handle data type")
	errUnexpectedDataType = fmt.Errorf("unexpected data type")
)

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

	ctx := context.Background()

	if len(opts.CollStatsCollections) > 0 {
		exp.collectors = append(exp.collectors, &collstatsCollector{ctx: ctx, client: client, collections: opts.CollStatsCollections})
	}

	exp.collectors = append(exp.collectors, &diagnosticDataCollector{ctx: ctx, client: client})
	exp.collectors = append(exp.collectors, &replSetGetStatusCollector{ctx: ctx, client: client})

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
		ErrorLog:      e.log,
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
