// mongodb_exporter
// Copyright (C) 2017 Percona LLC
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

package main

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/sirupsen/logrus"

	"github.com/percona/mongodb_exporter/exporter"
)

//nolint:gochecknoglobals
var (
	version   string
	commit    string
	buildDate string
)

// GlobalFlags has command line flags to configure the exporter.
type GlobalFlags struct {
	CollStatsNamespaces   string   `name:"mongodb.collstats-colls" help:"List of comma separared databases.collections to get $collStats" placeholder:"db1,db2.col2"`
	IndexStatsCollections string   `name:"mongodb.indexstats-colls" help:"List of comma separared databases.collections to get $indexStats" placeholder:"db1.col1,db2.col2"`
	URI                   []string `name:"mongodb.uri" help:"MongoDB connection URI" env:"MONGODB_URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	GlobalConnPool        bool     `name:"mongodb.global-conn-pool" help:"Use global connection pool instead of creating new pool for each http request." negatable:""`
	DirectConnect         bool     `name:"mongodb.direct-connect" help:"Whether or not a direct connect should be made. Direct connections are not valid if multiple hosts are specified or an SRV URI is used." default:"true" negatable:""`
	WebListenAddress      string   `name:"web.listen-address" help:"Address to listen on for web interface and telemetry" default:":9216"`
	WebTelemetryPath      string   `name:"web.telemetry-path" help:"Metrics expose path" default:"/metrics"`
	TLSConfigPath         string   `name:"web.config" help:"Path to the file having Prometheus TLS config for basic auth"`
	LogLevel              string   `name:"log.level" help:"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]" enum:"debug,info,warn,error,fatal" default:"error"`
	ConnectTimeoutMS      int      `name:"mongodb.connect-timeout-ms" help:"Connection timeout in milliseconds" default:"5000"`

	EnableDiagnosticData   bool `name:"collector.diagnosticdata" help:"Enable collecting metrics from getDiagnosticData"`
	EnableReplicasetStatus bool `name:"collector.replicasetstatus" help:"Enable collecting metrics from replSetGetStatus"`
	EnableDBStats          bool `name:"collector.dbstats" help:"Enable collecting metrics from dbStats"`
	EnableTopMetrics       bool `name:"collector.topmetrics" help:"Enable collecting metrics from top admin command"`
	EnableIndexStats       bool `name:"collector.indexstats" help:"Enable collecting metrics from $indexStats"`
	EnableCollStats        bool `name:"collector.collstats" help:"Enable collecting metrics from $collStats"`

	EnableOverrideDescendingIndex bool `name:"metrics.overridedescendingindex" help:"Enable descending index name override to replace -1 with _DESC"`

	CollectAll bool `name:"collect-all" help:"Enable all collectors. Same as specifying all --collector.<name>"`

	CollStatsLimit int `name:"collector.collstats-limit" help:"Disable collstats, dbstats, topmetrics and indexstats collector if there are more than <n> collections. 0=No limit" default:"0"`

	DiscoveringMode bool `name:"discovering-mode" help:"Enable autodiscover collections" negatable:""`
	CompatibleMode  bool `name:"compatible-mode" help:"Enable old mongodb-exporter compatible metrics" negatable:""`
	Version         bool `name:"version" help:"Show version and exit"`
}

func main() {
	var opts GlobalFlags
	_ = kong.Parse(&opts,
		kong.Name("mongodb_exporter"),
		kong.Description("MongoDB Prometheus exporter"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		})

	if opts.Version {
		fmt.Println("mongodb_exporter - MongoDB Prometheus exporter")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build date: %s\n", buildDate)
		return
	}

	log := logrus.New()

	levels := map[string]logrus.Level{
		"debug": logrus.DebugLevel,
		"error": logrus.ErrorLevel,
		"fatal": logrus.FatalLevel,
		"info":  logrus.InfoLevel,
		"warn":  logrus.WarnLevel,
	}
	log.SetLevel(levels[opts.LogLevel])

	log.Debugf("Compatible mode: %v", opts.CompatibleMode)

	if opts.WebTelemetryPath == "" {
		log.Warn("Web telemetry path \"\" invalid, falling back to \"/\" instead")
		opts.WebTelemetryPath = "/"
	}

	serverOpts := &exporter.ServerOpts{
		Path:             opts.WebTelemetryPath,
		MultiTargetPath:  "/scrape",
		WebListenAddress: opts.WebListenAddress,
		TLSConfigPath:    opts.TLSConfigPath,
	}
	exporter.RunWebServer(serverOpts, buildServers(opts, log), log)

}

func buildExporter(opts GlobalFlags, uri string, log *logrus.Logger) *exporter.Exporter {

	log.Debugf("Connection URI: %s", uri)

	exporterOpts := &exporter.Opts{
		CollStatsNamespaces:   strings.Split(opts.CollStatsNamespaces, ","),
		CompatibleMode:        opts.CompatibleMode,
		DiscoveringMode:       opts.DiscoveringMode,
		IndexStatsCollections: strings.Split(opts.IndexStatsCollections, ","),
		Logger:                log,
		URI:                   uri,
		GlobalConnPool:        opts.GlobalConnPool,
		DirectConnect:         opts.DirectConnect,
		ConnectTimeoutMS:      opts.ConnectTimeoutMS,

		EnableDiagnosticData:   opts.EnableDiagnosticData,
		EnableReplicasetStatus: opts.EnableReplicasetStatus,
		EnableTopMetrics:       opts.EnableTopMetrics,
		EnableDBStats:          opts.EnableDBStats,
		EnableIndexStats:       opts.EnableIndexStats,
		EnableCollStats:        opts.EnableCollStats,

		EnableOverrideDescendingIndex: opts.EnableOverrideDescendingIndex,

		CollStatsLimit: opts.CollStatsLimit,
		CollectAll:     opts.CollectAll,
	}

	e := exporter.New(exporterOpts)

	return e
}

func buildServers(opts GlobalFlags, log *logrus.Logger) []*exporter.Exporter {

	var servers []*exporter.Exporter

	if len(opts.URI) == 1 {
		opts.URI = strings.Split(opts.URI[0], " ")
	}
	for serverIdx := range opts.URI {
		URI := opts.URI[serverIdx]

		if !strings.HasPrefix(URI, "mongodb") {
			log.Debugf("Prepending mongodb:// to the URI %s", URI)
			URI = "mongodb://" + URI
		}

		servers = append(servers, buildExporter(opts, URI, log))
	}

	return servers
}
