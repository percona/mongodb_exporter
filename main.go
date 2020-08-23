// mongodb_exporter
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
	DSN                   string `name:"mongodb.dsn" help:"MongoDB connection URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	ExposePath            string `name:"expose-path" help:"Metrics expose path" default:"/metrics"`
	IndexStatsCollections string `name:"mongodb.indexstats-colls" help:"List of comma separared databases.collections to get $indexStats" placeholder:"db1.col1,db2.col2"`
	ExposePort            int    `name:"expose-port" help:"HTTP expose server port" default:"9216"`
	CompatibleMode        bool   `name:"compatible-mode" help:"Enable old mongodb-exporter compatible metrics"`
	Debug                 bool   `name:"debug" short:"D" help:"Enable debug mode"`
	Version               bool   `name:"version" help:"Show version and exit"`
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
		fmt.Println("mnogo-exporter - MongoDB Prometheus exporter")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build date: %s\n", buildDate)

		return
	}

	log := logrus.New()

	if opts.Debug {
		log.SetLevel(logrus.DebugLevel)
	}

	log.Debugf("Compatible mode: %v", opts.CompatibleMode)

	exporterOpts := &exporter.Opts{
		CollStatsCollections:  strings.Split(opts.CollStatsCollections, ","),
		IndexStatsCollections: strings.Split(opts.CollStatsCollections, ","),
		CompatibleMode:        opts.CompatibleMode,
		DSN:                   opts.DSN,
		Path:                  opts.ExposePath,
		Port:                  opts.ExposePort,
		Logger:                log,
	}

	e, err := exporter.New(exporterOpts)
	if err != nil {
		log.Fatal(err)
	}

	e.Run()
}
