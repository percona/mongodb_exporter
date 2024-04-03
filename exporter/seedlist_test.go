package exporter

import (
	"net"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestGetSeedListFromSRV(t *testing.T) {
	log := logrus.New()
	srv := tu.SetupFakeResolver()

	defer srv.Close()
	defer mockdns.UnpatchNet(net.DefaultResolver)

	tests := map[string]string{
		"mongodb+srv://server.example.com":                                         "mongodb://mongo1.example.com:17001,mongo2.example.com:17002,mongo3.example.com:17003/?authSource=admin",
		"mongodb+srv://user:pass@server.example.com?replicaSet=rs0&authSource=db0": "mongodb://user:pass@mongo1.example.com:17001,mongo2.example.com:17002,mongo3.example.com:17003/?authSource=db0&replicaSet=rs0",
		"mongodb+srv://unexistent.com":                                             "mongodb+srv://unexistent.com",
	}

	for uri, expected := range tests {
		actual := GetSeedListFromSRV(uri, log)
		assert.Equal(t, expected, actual)
	}
}
