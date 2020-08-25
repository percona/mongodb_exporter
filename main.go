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

package main

import (
	"fmt"
	"os"
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
	CollStatsCollections  string `name:"mongodb.collstats-colls" help:"List of comma separared databases.collections to get $collStats" placeholder:"db1.col1,db2.col2"`
	URI                   string `name:"mongodb.uri" help:"MongoDB connection URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	WebTelemetryPath      string `name:"web.telemetry-path" help:"Metrics expose path" default:"/metrics"`
	IndexStatsCollections string `name:"mongodb.indexstats-colls" help:"List of comma separared databases.collections to get $indexStats" placeholder:"db1.col1,db2.col2"`
	ExposePort            int    `name:"expose-port" help:"HTTP expose server port" default:"9216"`
	CompatibleMode        bool   `name:"compatible-mode" help:"Enable old mongodb-exporter compatible metrics" default:"true"`
	Version               bool   `name:"version" help:"Show version and exit"`

	// To make this exporter a drop-in replacement of origimal one
	LogLevel             string `name:"log.level" help:"Only log messages with the given severuty or above. Valid levels: [debbug, info, warn, error, fatal]" enum:"debbug,info,warn,error,fatal" default:"error"`
	Test                 bool   `name:"test" help:"Check MongoDB connection, print BuildInfo() information and exit. (Not implemented yet)"`
	CollectCollection    bool   `name:"collect.collection" help:"Enable collection of Collection metrics. (Deprecated. Use --mongodb.collstats-colls)"`
	CollectDatabase      bool   `name:"collect.database" help:"Enable collection of Database metrics. (Deprecated)"`
	CollectTopMetrics    bool   `name:"collect.topmetrics" help:"Enable collection of table top metrics (Deprecated)"`
	NoCollectIndexUsage  bool   `name:"no-collect.indexusage" help:"Enable collection of per index usage stats (Deprecated. Use --mongodb.indexstats-colls)"`
	NoCollecConPoolStats bool   `name:"no-collect.connpoolstats" help:"Enable collection of connection pool stats. (Deprecated)"`
	WebListenAddress     string `name:"web.listen-address" help:"Address to listen on for web interface and telemetry" default:":9216"`
}

func main() {
	var opts GlobalFlags
	_ = kong.Parse(&opts,
		kong.Name("mnogo_exporter"),
		kong.Description("MongoDB Prometheus exporter"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		})

	if opts.Version {
		fmt.Println("mnogo-exporter - MongoDB Prometheus exporter")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build date: %s\n", buildDate)

		return
	}

	log := logrus.New()

	levels := map[string]logrus.Level{
		"debbug": logrus.DebugLevel,
		"error":  logrus.ErrorLevel,
		"fatal":  logrus.FatalLevel,
		"info":   logrus.InfoLevel,
		"warn":   logrus.WarnLevel,
	}
	log.SetLevel(levels[opts.LogLevel])

	log.Debugf("Compatible mode: %v", opts.CompatibleMode)

	if opts.URI == "" {
		opts.URI = os.Getenv("MONGODB_URI")
	}

	if !strings.HasPrefix(opts.URI, "mongodb") && !strings.HasPrefix(opts.URI, "mongodb+srv") {
		opts.URI = "mongodb://" + opts.URI
	}

	exporterOpts := &exporter.Opts{
		CollStatsCollections:  strings.Split(opts.CollStatsCollections, ","),
		IndexStatsCollections: strings.Split(opts.CollStatsCollections, ","),
		CompatibleMode:        opts.CompatibleMode,
		URI:                   opts.URI,
		Path:                  opts.WebTelemetryPath,
		Logger:                log,
		WebListenAddress:      opts.WebListenAddress,
	}

	e, err := exporter.New(exporterOpts)
	if err != nil {
		log.Fatal(err)
	}

	e.Run()
}
