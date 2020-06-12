package main

import (
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/sirupsen/logrus"

	"github.com/Percona-Lab/mnogo_exporter/exporter"
)

var (
	version   string
	commit    string //nolint
	buildDate string //nolint
)

// GlobalFlags has command line flags to configure the exporter.
type GlobalFlags struct {
	Debug      bool   `name:"debug" short:"D" help:"Enable debug mode"`
	DSN        string `name:"mongodb.dsn" help:"MongoDB connection URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	ExposePath string `name:"expose-path" help:"Metrics expose path" default:"/metrics"`
	ExposePort int    `name:"expose-port" help:"HTTP expose server port" default:"9216"`
	Version    bool   `name:"version" help:"Show version and exit"`
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

	if opts.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	exporterOpts := &exporter.Opts{
		DSN:  opts.DSN,
		Log:  log,
		Path: opts.ExposePath,
		Port: opts.ExposePort,
	}

	e, err := exporter.New(exporterOpts)
	if err != nil {
		log.Fatal(err)
	}

	e.Run()
}
