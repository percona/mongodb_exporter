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
	User                  string   `name:"mongodb.user" help:"monitor user, need clusterMonitor role in admin db and read role in local db" env:"MONGODB_USER" placeholder:"monitorUser"`
	Password              string   `name:"mongodb.password" help:"monitor user password" env:"MONGODB_PASSWORD" placeholder:"monitorPassword"`
	CollStatsNamespaces   string   `name:"mongodb.collstats-colls" help:"List of comma separared databases.collections to get $collStats" placeholder:"db1,db2.col2"`
	IndexStatsCollections string   `name:"mongodb.indexstats-colls" help:"List of comma separared databases.collections to get $indexStats" placeholder:"db1.col1,db2.col2"`
	URI                   []string `name:"mongodb.uri" help:"MongoDB connection URI" env:"MONGODB_URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	GlobalConnPool        bool     `name:"mongodb.global-conn-pool" help:"Use global connection pool instead of creating new pool for each http request." negatable:""`
	DirectConnect         bool     `name:"mongodb.direct-connect" help:"Whether or not a direct connect should be made. Direct connections are not valid if multiple hosts are specified or an SRV URI is used." default:"true" negatable:""`
	WebListenAddress      string   `name:"web.listen-address" help:"Address to listen on for web interface and telemetry" default:":9216"`
	WebTelemetryPath      string   `name:"web.telemetry-path" help:"Metrics expose path" default:"/metrics"`
	TLSConfigPath         string   `name:"web.config" help:"Path to the file having Prometheus TLS config for basic auth"`
	TimeoutOffset         int      `name:"web.timeout-offset" help:"Offset to subtract from the request timeout in seconds" default:"1"`
	LogLevel              string   `name:"log.level" help:"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]" enum:"debug,info,warn,error,fatal" default:"error"`
	ConnectTimeoutMS      int      `name:"mongodb.connect-timeout-ms" help:"Connection timeout in milliseconds" default:"5000"`

	EnableExporterMetrics    bool `name:"collector.exporter-metrics" help:"Enable collecting metrics about the exporter itself (process_*, go_*)" negatable:"" default:"True"`
	EnableDiagnosticData     bool `name:"collector.diagnosticdata" help:"Enable collecting metrics from getDiagnosticData"`
	EnableReplicasetStatus   bool `name:"collector.replicasetstatus" help:"Enable collecting metrics from replSetGetStatus"`
	EnableReplicasetConfig   bool `name:"collector.replicasetconfig" help:"Enable collecting metrics from replSetGetConfig"`
	EnableDBStats            bool `name:"collector.dbstats" help:"Enable collecting metrics from dbStats"`
	EnableDBStatsFreeStorage bool `name:"collector.dbstatsfreestorage" help:"Enable collecting free space metrics from dbStats"`
	EnableTopMetrics         bool `name:"collector.topmetrics" help:"Enable collecting metrics from top admin command"`
	EnableCurrentopMetrics   bool `name:"collector.currentopmetrics" help:"Enable collecting metrics currentop admin command"`
	EnableIndexStats         bool `name:"collector.indexstats" help:"Enable collecting metrics from $indexStats"`
	EnableCollStats          bool `name:"collector.collstats" help:"Enable collecting metrics from $collStats"`
	EnableProfile            bool `name:"collector.profile" help:"Enable collecting metrics from profile"`
	EnableFCV                bool `name:"collector.fcv" help:"Enable Feature Compatibility Version collector"`
	EnableShards             bool `help:"Enable collecting metrics from sharded Mongo clusters about chunks" name:"collector.shards"`
	EnablePBM                bool `help:"Enable collecting metrics from Percona Backup for MongoDB" name:"collector.pbm"`

	EnableOverrideDescendingIndex bool `name:"metrics.overridedescendingindex" help:"Enable descending index name override to replace -1 with _DESC"`

	CollectAll bool `name:"collect-all" help:"Enable all collectors. Same as specifying all --collector.<name>"`

	CollStatsLimit         int  `name:"collector.collstats-limit" help:"Disable collstats, dbstats, topmetrics and indexstats collector if there are more than <n> collections. 0=No limit" default:"0"`
	CollStatsEnableDetails bool `name:"collector.collstats-enable-details" help:"Enable collecting index details and wired tiger metrics from $collStats" default:"false"`

	ProfileTimeTS int `name:"collector.profile-time-ts" help:"Set time for scrape slow queries." default:"30"`

	CurrentOpSlowTime string `name:"collector.currentopmetrics-slow-time" help:"Set minimum time for registration queries." default:"1m"`

	DiscoveringMode bool `name:"discovering-mode" help:"Enable autodiscover collections" negatable:""`
	CompatibleMode  bool `name:"compatible-mode" help:"Enable old mongodb-exporter compatible metrics" negatable:""`
	Version         bool `name:"version" help:"Show version and exit"`
	SplitCluster    bool `name:"split-cluster" help:"Treat each node in cluster as a separate target" negatable:"" default:"false"`
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

		DisableDefaultRegistry:   !opts.EnableExporterMetrics,
		EnableDiagnosticData:     opts.EnableDiagnosticData,
		EnableReplicasetStatus:   opts.EnableReplicasetStatus,
		EnableReplicasetConfig:   opts.EnableReplicasetConfig,
		EnableCurrentopMetrics:   opts.EnableCurrentopMetrics,
		EnableTopMetrics:         opts.EnableTopMetrics,
		EnableDBStats:            opts.EnableDBStats,
		EnableDBStatsFreeStorage: opts.EnableDBStatsFreeStorage,
		EnableIndexStats:         opts.EnableIndexStats,
		EnableCollStats:          opts.EnableCollStats,
		EnableProfile:            opts.EnableProfile,
		EnableShards:             opts.EnableShards,
		EnableFCV:                opts.EnableFCV,
		EnablePBMMetrics:         opts.EnablePBM,

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
