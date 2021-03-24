package main

import (
	"testing"

	"github.com/percona/mongodb_exporter/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildExporter(t *testing.T) {
	opts := GlobalFlags{
		CollStatsCollections:  "c1,c2,c3",
		IndexStatsCollections: "i1,i2,i3",
		URI:                   "mongodb://usr:pwd@127.0.0.1/",
		GlobalConnPool:        false, // to avoid testing the connection
		WebListenAddress:      "localhost:12345",
		WebTelemetryPath:      "/mymetrics",
		LogLevel:              "debug",

		DisableDiagnosticData:   true,
		DisableReplicasetStatus: true,

		CompatibleMode: true,
	}
	MongoInstances := []*config.MongoInstance{
		{
			Name:    "",
			Host:    "",
			Port:    "",
			Account: []*config.Account{
				{
					Username: "",
					Password: "",
				},
			},
		},
	}
	conf := &config.Config{
		MongoInstance: MongoInstances,
	}

	_, err := buildExporter(opts, conf)
	assert.NoError(t, err)
}
