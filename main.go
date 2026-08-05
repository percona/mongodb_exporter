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
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

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

var errNoMongoTargets = errors.New("no MongoDB targets configured: specify --mongodb.uri, MONGODB_URI, or --config.file with auth_modules")

// GlobalFlags has command line flags to configure the exporter.
type GlobalFlags struct {
	ConfigFile string `help:"Path to the dynamic target authentication config file" name:"config.file"`

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

	authConfig, err := loadAuthConfig(opts.ConfigFile)
	if err != nil {
		ctx.Fatalf("Cannot load auth config: %v", err)
	}
	err = validateTargetConfiguration(opts, authConfig)
	if err != nil {
		ctx.FatalIfErrorf(err)
	}

	if opts.TimeoutOffset <= 0 {
		logger.Warn("Timeout offset needs to be greater than \"0\", falling back to \"1\". You can specify the timout offset with --web.timeout-offset command argument")
		opts.TimeoutOffset = 1
	}

	serverOpts := &exporter.ServerOpts{
		Path:                   opts.WebTelemetryPath,
		MultiTargetPath:        "/scrape",
		DynamicTargetPath:      "/probe",
		OverallTargetPath:      "/scrapeall",
		WebListenAddress:       opts.WebListenAddress,
		TLSConfigPath:          opts.TLSConfigPath,
		DynamicTargetFactory:   newDynamicTargetFactory(opts, authConfig, logger),
		DisableDefaultRegistry: !opts.EnableExporterMetrics,
	}
	exporter.RunWebServer(serverOpts, buildServers(opts, logger), logger)
}

func validateTargetConfiguration(opts GlobalFlags, config authConfig) error {
	if len(opts.URI) == 0 && len(config.AuthModules) == 0 {
		return errNoMongoTargets
	}

	return nil
}

func newDynamicTargetFactory(opts GlobalFlags, config authConfig, logger *slog.Logger) exporter.DynamicTargetFactory {
	if len(config.AuthModules) == 0 {
		return nil
	}

	var mu sync.Mutex
	// ponytail: service-discovery targets are stable; add bounded eviction only if target churn becomes measurable.
	handlers := make(map[string]http.Handler)

	return func(target, authModule string) (http.Handler, error) {
		module, err := resolveAuthModule(config.AuthModules, authModule)
		if err != nil {
			return nil, err
		}

		cacheKey := authModule + "\x00" + target
		mu.Lock()
		defer mu.Unlock()
		if handler, ok := handlers[cacheKey]; ok {
			return handler, nil
		}

		dynamicOpts := opts
		dynamicOpts.User = ""
		dynamicOpts.Password = ""
		dynamicOpts.URI = nil
		handler := buildExporter(dynamicOpts, buildDynamicURI(target, module), logger).Handler()
		handlers[cacheKey] = handler

		return handler, nil
	}
}

func buildExporter(opts GlobalFlags, uri string, log *slog.Logger) *exporter.Exporter {
	uri = buildURI(uri, opts.User, opts.Password)
	log.Debug("Connection URI", "uri", redactMongoURI(uri))

	uriParsed, _ := url.Parse(uri)
	var nodeName string
	switch {
	case uriParsed == nil:
		nodeName = ""
	case uriParsed.Port() != "":
		nodeName = net.JoinHostPort(uriParsed.Hostname(), uriParsed.Port())
	default:
		nodeName = uriParsed.Host
	}

	collStatsNamespaces := []string{}
	if opts.CollStatsNamespaces != "" {
		collStatsNamespaces = strings.Split(opts.CollStatsNamespaces, ",")
	}
	indexStatsCollections := []string{}
	if opts.IndexStatsCollections != "" {
		indexStatsCollections = strings.Split(opts.IndexStatsCollections, ",")
	}
	exporterOpts := &exporter.Opts{
		CollStatsNamespaces:   collStatsNamespaces,
		CompatibleMode:        opts.CompatibleMode,
		DiscoveringMode:       opts.DiscoveringMode,
		IndexStatsCollections: indexStatsCollections,
		Logger:                log,
		URI:                   uri,
		NodeName:              nodeName,
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
	}

	return exporter.New(exporterOpts)
}

func redactMongoURI(rawURI string) string {
	uri, err := url.Parse(rawURI)
	if err != nil {
		return "<invalid MongoDB URI>"
	}

	return uri.Redacted()
}

func buildServers(opts GlobalFlags, logger *slog.Logger) []*exporter.Exporter {
	URIs := parseURIList(opts.URI, logger, opts.SplitCluster)
	servers := make([]*exporter.Exporter, len(URIs))
	for serverIdx := range URIs {
		servers[serverIdx] = buildExporter(opts, URIs[serverIdx], logger)
	}

	return servers
}

func parseURIList(uriList []string, logger *slog.Logger, splitCluster bool) []string { //nolint:gocognit,cyclop
	var URIs []string

	// If server URI is prefixed with mongodb scheme string, then every next URI in
	// line not prefixed with mongodb scheme string is a part of cluster. Otherwise,
	// treat it as a standalone server
	realURI := ""
	matchRegexp := regexp.MustCompile(`^mongodb(\+srv)?://`)
	for _, URI := range uriList {
		matches := matchRegexp.FindStringSubmatch(URI)
		if matches != nil {
			if realURI != "" {
				// Add the previous host buffer to the url list as we met the scheme part
				URIs = append(URIs, realURI)
				realURI = ""
			}
			if matches[1] == "" {
				realURI = URI
			} else {
				// There can be only one host in SRV connection string
				if splitCluster {
					// In splitCluster mode we get srv connection string from SRV recors
					URI = exporter.GetSeedListFromSRV(URI, logger)
				}
				URIs = append(URIs, URI)
			}
		} else {
			if realURI == "" {
				URIs = append(URIs, "mongodb://"+URI)
			} else {
				realURI += "," + URI
			}
		}
	}
	if realURI != "" {
		URIs = append(URIs, realURI)
	}

	if splitCluster {
		// In this mode we split cluster strings into separate targets
		separateURIs := []string{}
		for _, hosturl := range URIs {
			urlParsed, err := url.Parse(hosturl)
			if err != nil {
				log.Fatalf("Failed to parse URI %s: %v", hosturl, err)
			}
			for _, host := range strings.Split(urlParsed.Host, ",") {
				targetURI := "mongodb://"
				if urlParsed.User != nil {
					targetURI += urlParsed.User.String() + "@"
				}
				targetURI += host
				if urlParsed.Path != "" {
					targetURI += urlParsed.Path
				}
				if urlParsed.RawQuery != "" {
					targetURI += "?" + urlParsed.RawQuery
				}
				separateURIs = append(separateURIs, targetURI)
			}
		}
		return separateURIs
	}
	return URIs
}

// buildURIManually builds the URI manually by checking if the user and password are supplied
func buildURIManually(uri string, user string, password string) string {
	uriArray := strings.SplitN(uri, "://", 2) //nolint:mnd
	prefix := uriArray[0] + "://"
	uri = uriArray[1]

	// IF user@pass not contained in uri AND custom user and pass supplied in arguments
	// DO concat a new uri with user and pass arguments value
	if !strings.Contains(uri, "@") && user != "" && password != "" {
		// add user and pass to the uri
		uri = fmt.Sprintf("%s:%s@%s", user, password, uri)
	}

	// add back prefix after adding the user and pass
	uri = prefix + uri

	return uri
}

func buildURI(uri string, user string, password string) string {
	defaultPrefix := "mongodb://" // default prefix

	if !strings.HasPrefix(uri, defaultPrefix) && !strings.HasPrefix(uri, "mongodb+srv://") {
		uri = defaultPrefix + uri
	}
	parsedURI, err := url.Parse(uri)
	if err != nil {
		// PMM generates URI with escaped path to socket file, so url.Parse fails
		// in this case we build URI manually
		return buildURIManually(uri, user, password)
	}

	if parsedURI.User == nil && user != "" && password != "" {
		parsedURI.User = url.UserPassword(user, password)
	}

	return parsedURI.String()
}
