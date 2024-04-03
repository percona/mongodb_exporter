package exporter

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/percona/mongodb_exporter/internal/tu"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func setupFakeResolver() *mockdns.Server {

	p1, err1 := strconv.ParseInt(tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"), 10, 64)
	p2, err2 := strconv.ParseInt(tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "17002"), 10, 64)
	p3, err3 := strconv.ParseInt(tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY2_PORT", "17003"), 10, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		panic("Invalid ports")
	}

	var testZone = map[string]mockdns.Zone{
		"_mongodb._tcp.server.example.com.": {
			SRV: []net.SRV{
				{
					Target: "mongo1.example.com.",
					Port:   uint16(p1),
				},
				{
					Target: "mongo2.example.com.",
					Port:   uint16(p2),
				},
				{
					Target: "mongo3.example.com.",
					Port:   uint16(p3),
				},
			},
		},
		"server.example.com.": {
			TXT: []string{"authSource=admin"},
			A:   []string{"1.2.3.4"},
		},
		"mongo1.example.com.": {
			A: []string{"127.0.0.1"},
		},
		"mongo2.example.com.": {
			A: []string{"127.0.0.1"},
		},
		"mongo3.example.com.": {
			A: []string{"127.0.0.1"},
		},
	}

	srv, _ := mockdns.NewServer(testZone, true)
	srv.PatchNet(net.DefaultResolver)

	return srv
}

func TestGetSeedListFromSRV(t *testing.T) {
	t.Parallel()

	log := logrus.New()
	srv := setupFakeResolver()

	defer srv.Close()
	defer mockdns.UnpatchNet(net.DefaultResolver)

	tests := map[string]string{
		"mongodb+srv://server.example.com":                                         "mongodb://mongo1.example.com:17001,mongo2.example.com:17002,mongo3.example.com:17003/?authSource=admin",
		"mongodb+srv://user:pass@server.example.com?replicaSet=rs0&authSource=db0": "mongodb://user:pass@mongo1.example.com:17001,mongo2.example.com:17002,mongo3.example.com:17003/?authSource=db0&replicaSet=rs0",
		"mongodb+srv://google.com":                                                 "mongodb+srv://google.com",
	}

	for uri, expected := range tests {
		actual := GetSeedListFromSRV(uri, log)
		fmt.Println(actual)
		assert.Equal(t, expected, actual)
	}

}
