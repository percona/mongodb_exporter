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
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/prometheus/common/promslog"

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
	User                  string   `env:"MONGODB_USER"                                                                    help:"monitor user, need clusterMonitor role in admin db and read role in local db"                                                            name:"mongodb.user"                                                                                        placeholder:"monitorUser"`
	Password              string   `env:"MONGODB_PASSWORD"                                                                help:"monitor user password"                                                                                                                   name:"mongodb.password"                                                                                    placeholder:"monitorPassword"`
	CollStatsNamespaces   string   `help:"List of comma separared databases.collections to get $collStats"                name:"mongodb.collstats-colls"                                                                                                                 placeholder:"db1,db2.col2"`
	IndexStatsCollections string   `help:"List of comma separared databases.collections to get $indexStats"               name:"mongodb.indexstats-colls"                                                                                                                placeholder:"db1.col1,db2.col2"`
	URI                   []string `env:"MONGODB_URI"                                                                     help:"MongoDB connection URI"                                                                                                                  name:"mongodb.uri"                                                                                         placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	GlobalConnPool        bool     `help:"Use global connection pool instead of creating new pool for each http request." name:"mongodb.global-conn-pool"                                                                                                                negatable:""`
	DirectConnect         bool     `default:"true"                                                                        help:"Whether or not a direct connect should be made. Direct connections are not valid if multiple hosts are specified or an SRV URI is used." name:"mongodb.direct-connect"                                                                              negatable:""`
	WebListenAddress      string   `default:":9216"                                                                       help:"Address to listen on for web interface and telemetry"                                                                                    name:"web.listen-address"`
	WebTelemetryPath      string   `default:"/metrics"                                                                    help:"Metrics expose path"                                                                                                                     name:"web.telemetry-path"`
	TLSConfigPath         string   `help:"Path to the file having Prometheus TLS config for basic auth"                   name:"web.config"`
	TimeoutOffset         int      `default:"1"                                                                           help:"Offset to subtract from the request timeout in seconds"                                                                                  name:"web.timeout-offset"`
	LogLevel              string   `default:"error"                                                                       enum:"debug,info,warn,error,fatal"                                                                                                             help:"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]" name:"log.level"`
	ConnectTimeoutMS      int      `default:"5000"                                                                        help:"Connection timeout in milliseconds"                                                                                                      name:"mongodb.connect-timeout-ms"`

	EnableExporterMetrics          bool `default:"True"                                                            help:"Enable collecting metrics about the exporter itself (process_*, go_*)" name:"collector.exporter-metrics" negatable:""`
	EnableDiagnosticData           bool `help:"Enable collecting metrics from getDiagnosticData"                   name:"collector.diagnosticdata"`
	EnableDiagnosticDataHistograms bool `help:"Enable collecting histogram bucket metrics from getDiagnosticData"  name:"collector.diagnosticdata-histograms"`
	EnableReplicasetStatus         bool `help:"Enable collecting metrics from replSetGetStatus"                    name:"collector.replicasetstatus"`
	EnableReplicasetConfig         bool `help:"Enable collecting metrics from replSetGetConfig"                    name:"collector.replicasetconfig"`
	EnableDBStats                  bool `help:"Enable collecting metrics from dbStats"                             name:"collector.dbstats"`
	EnableDBStatsFreeStorage       bool `help:"Enable collecting free space metrics from dbStats"                  name:"collector.dbstatsfreestorage"`
	EnableTopMetrics               bool `help:"Enable collecting metrics from top admin command"                   name:"collector.topmetrics"`
	EnableCurrentopMetrics         bool `help:"Enable collecting metrics currentop admin command"                  name:"collector.currentopmetrics"`
	EnableIndexStats               bool `help:"Enable collecting metrics from $indexStats"                         name:"collector.indexstats"`
	EnableCollStats                bool `help:"Enable collecting metrics from $collStats"                          name:"collector.collstats"`
	EnableProfile                  bool `help:"Enable collecting metrics from profile"                             name:"collector.profile"`
	EnableFCV                      bool `help:"Enable Feature Compatibility Version collector"                     name:"collector.fcv"`
	EnableShards                   bool `help:"Enable collecting metrics from sharded Mongo clusters about chunks" name:"collector.shards"`
	EnablePBM                      bool `help:"Enable collecting metrics from Percona Backup for MongoDB"          name:"collector.pbm"`

	EnableOverrideDescendingIndex bool `help:"Enable descending index name override to replace -1 with _DESC" name:"metrics.overridedescendingindex"`

	CollectAll bool `help:"Enable all collectors. Same as specifying all --collector.<name>" name:"collect-all"`

	CollStatsLimit         int  `default:"0"     help:"Disable collstats, dbstats, topmetrics and indexstats collector if there are more than <n> collections. 0=No limit" name:"collector.collstats-limit"`
	CollStatsEnableDetails bool `default:"false" help:"Enable collecting index details and wired tiger metrics from $collStats"                                            name:"collector.collstats-enable-details"`

	ProfileTimeTS int `default:"30" help:"Set time for scrape slow queries." name:"collector.profile-time-ts"`

	CurrentOpSlowTime string `default:"5m" help:"Set minimum time for registration queries." name:"collector.currentopmetrics-slow-time"`

	DiscoveringMode bool `help:"Enable autodiscover collections"                name:"discovering-mode"                                negatable:""`
	CompatibleMode  bool `help:"Enable old mongodb-exporter compatible metrics" name:"compatible-mode"                                 negatable:""`
	Version         bool `help:"Show version and exit"                          name:"version"`
	SplitCluster    bool `default:"false"                                       help:"Treat each node in cluster as a separate target" name:"split-cluster" negatable:""`
}

func main() {
	var opts GlobalFlags
	ctx := kong.Parse(&opts,
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

	logLevel := promslog.NewLevel()
	_ = logLevel.Set(opts.LogLevel)
	logger := promslog.New(&promslog.Config{
		Level: logLevel,
	})
	logger.Debug("Compatible mode", "compatible_mode", opts.CompatibleMode)

	if opts.WebTelemetryPath == "" {
		logger.Warn("Web telemetry path \"\" is invalid, falling back to \"/\" instead")
		opts.WebTelemetryPath = "/"
	}

	if len(opts.URI) == 0 {
		ctx.Printf("No MongoDB hosts specified. You can specify the host(s) with the --mongodb.uri command argument or the MONGODB_URI environment variable")
	}

	if opts.TimeoutOffset <= 0 {
		logger.Warn("Timeout offset needs to be greater than \"0\", falling back to \"1\". You can specify the timout offset with --web.timeout-offset command argument")
		opts.TimeoutOffset = 1
	}

	serverOpts := &exporter.ServerOpts{
		Path:              opts.WebTelemetryPath,
		MultiTargetPath:   "/scrape",
		OverallTargetPath: "/scrapeall",
		WebListenAddress:  opts.WebListenAddress,
		TLSConfigPath:     opts.TLSConfigPath,
	}

	exporterOpts := buildOpts(opts)

	exporter.RunWebServer(serverOpts, buildServers(opts, logger, exporterOpts), exporterOpts, logger)
}

func buildOpts(opts GlobalFlags) *exporter.Opts {
	collStatsNamespaces := []string{}
	if opts.CollStatsNamespaces != "" {
		collStatsNamespaces = strings.Split(opts.CollStatsNamespaces, ",")
	}
	indexStatsCollections := []string{}
	if opts.IndexStatsCollections != "" {
		indexStatsCollections = strings.Split(opts.IndexStatsCollections, ",")
	}

	return &exporter.Opts{
		CollStatsNamespaces:   collStatsNamespaces,
		CompatibleMode:        opts.CompatibleMode,
		DiscoveringMode:       opts.DiscoveringMode,
		IndexStatsCollections: indexStatsCollections,
		GlobalConnPool:        opts.GlobalConnPool,
		DirectConnect:         opts.DirectConnect,
		ConnectTimeoutMS:      opts.ConnectTimeoutMS,
		TimeoutOffset:         opts.TimeoutOffset,

		DisableDefaultRegistry:         !opts.EnableExporterMetrics,
		EnableDiagnosticData:           opts.EnableDiagnosticData,
		EnableDiagnosticDataHistograms: opts.EnableDiagnosticDataHistograms,
		EnableReplicasetStatus:         opts.EnableReplicasetStatus,
		EnableReplicasetConfig:         opts.EnableReplicasetConfig,
		EnableCurrentopMetrics:         opts.EnableCurrentopMetrics,
		EnableTopMetrics:               opts.EnableTopMetrics,
		EnableDBStats:                  opts.EnableDBStats,
		EnableDBStatsFreeStorage:       opts.EnableDBStatsFreeStorage,
		EnableIndexStats:               opts.EnableIndexStats,
		EnableCollStats:                opts.EnableCollStats,
		EnableProfile:                  opts.EnableProfile,
		EnableShards:                   opts.EnableShards,
		EnableFCV:                      opts.EnableFCV,
		EnablePBMMetrics:               opts.EnablePBM,

		EnableOverrideDescendingIndex: opts.EnableOverrideDescendingIndex,

		CollStatsLimit:         opts.CollStatsLimit,
		CollStatsEnableDetails: opts.CollStatsEnableDetails,
		CollectAll:             opts.CollectAll,
		ProfileTimeTS:          opts.ProfileTimeTS,
		CurrentOpSlowTime:      opts.CurrentOpSlowTime,

		User:     opts.User,
		Password: opts.Password,
	}
}

func buildExporter(baseOpts *exporter.Opts, uri string, log *slog.Logger) *exporter.Exporter {
	uri = exporter.BuildURI(uri, baseOpts.User, baseOpts.Password)
	log.Debug("Connection URI", "uri", uri)

	uriParsed, _ := url.Parse(uri)
	var nodeName string
	if uriParsed != nil {
		if uriParsed.Port() != "" {
			nodeName = net.JoinHostPort(uriParsed.Hostname(), uriParsed.Port())
		} else {
			nodeName = uriParsed.Host
		}
	}

	exporterOpts := *baseOpts
	exporterOpts.URI = uri
	exporterOpts.Logger = log
	exporterOpts.NodeName = nodeName

	return exporter.New(&exporterOpts)
}

func buildServers(opts GlobalFlags, logger *slog.Logger, baseOpts *exporter.Opts) []*exporter.Exporter {
	if len(opts.URI) == 0 {
		return []*exporter.Exporter{}
	}

	URIs := exporter.ParseURIList(opts.URI, logger, opts.SplitCluster)
	servers := make([]*exporter.Exporter, len(URIs))
	for i, uri := range URIs {
		servers[i] = buildExporter(baseOpts, uri, logger)
	}

	return servers
}
