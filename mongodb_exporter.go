// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	pmmVersion "github.com/percona/pmm/version"

	"github.com/percona/mongodb_exporter/collector"
	"github.com/percona/mongodb_exporter/shared"
)

const (
	program           = "mongodb_exporter"
	versionDataFormat = "20060102-15:04:05"
)

var (
	listenAddressF = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9216").String()
	metricsPathF   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	collectDatabaseF             = kingpin.Flag("collect.database", "Enable collection of Database metrics").Bool()
	collectCollectionF           = kingpin.Flag("collect.collection", "Enable collection of Collection metrics").Bool()
	collectTopF                  = kingpin.Flag("collect.topmetrics", "Enable collection of table top metrics").Bool()
	collectIndexUsageF           = kingpin.Flag("collect.indexusage", "Enable collection of per index usage stats").Bool()
	mongodbCollectConnPoolStatsF = kingpin.Flag("collect.connpoolstats", "Collect MongoDB connpoolstats").Bool()

	uriF = kingpin.Flag("mongodb.uri", "MongoDB URI, format").
		PlaceHolder("[mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]").
		Default("mongodb://localhost:27017").
		Envar("MONGODB_URI").
		String()
	testF = kingpin.Flag("test", "Check MongoDB connection, print buildInfo() information and exit.").Bool()
)

func main() {
	initVersionInfo()
	log.AddFlags(kingpin.CommandLine)
	kingpin.Parse()

	if *testF {
		buildInfo, err := shared.TestConnection(
			shared.MongoSessionOpts{
				URI: *uriF,
			},
		)
		if err != nil {
			log.Errorf("Can't connect to MongoDB: %s", err)
			os.Exit(1)
		}
		fmt.Println(string(buildInfo))
		os.Exit(0)
	}

	// TODO: Maybe we should move version.Info() and version.BuildContext() to https://github.com/percona/exporter_shared
	// See: https://jira.percona.com/browse/PMM-3250 and https://github.com/percona/mongodb_exporter/pull/132#discussion_r262227248
	log.Infoln("Starting", program, version.Info())
	log.Infoln("Build context", version.BuildContext())

	programCollector := version.NewCollector(program)
	mongodbCollector := collector.NewMongodbCollector(&collector.MongodbCollectorOpts{
		URI:                      *uriF,
		CollectDatabaseMetrics:   *collectDatabaseF,
		CollectCollectionMetrics: *collectCollectionF,
		CollectTopMetrics:        *collectTopF,
		CollectIndexUsageStats:   *collectIndexUsageF,
		CollectConnPoolStats:     *mongodbCollectConnPoolStatsF,
	})
	prometheus.MustRegister(programCollector, mongodbCollector)

	promHandler := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
	exporter_shared.RunServer("MongoDB", *listenAddressF, *metricsPathF, promHandler)
}

// initVersionInfo sets version info
// If binary was build for PMM with environment variable PMM_RELEASE_VERSION
// `--version` will be displayed in PMM format. Also `PMM Version` will be connected
// to application version and will be printed in all logs.
// TODO: Refactor after moving version.Info() and version.BuildContext() to https://github.com/percona/exporter_shared
// See: https://jira.percona.com/browse/PMM-3250 and https://github.com/percona/mongodb_exporter/pull/132#discussion_r262227248
func initVersionInfo() {
	version.Version = pmmVersion.Version
	version.Revision = pmmVersion.FullCommit
	version.Branch = pmmVersion.Branch

	if buildDate, err := strconv.ParseInt(pmmVersion.Timestamp, 10, 64); err != nil {
		version.BuildDate = time.Unix(0, 0).Format(versionDataFormat)
	} else {
		version.BuildDate = time.Unix(buildDate, 0).Format(versionDataFormat)
	}

	if pmmVersion.PMMVersion != "" {
		version.Version += "-pmm-" + pmmVersion.PMMVersion
		kingpin.Version(pmmVersion.FullInfo())
	} else {
		kingpin.Version(version.Print(program))
	}

	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.Help = fmt.Sprintf("%s exports various MongoDB metrics in Prometheus format.\n", pmmVersion.ShortInfo())
}
