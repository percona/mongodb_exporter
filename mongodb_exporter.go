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

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/percona/mongodb_exporter/collector"
	"github.com/percona/mongodb_exporter/shared"
)

const (
	program = "mongodb_exporter"
)

var (
	versionF       = kingpin.Flag("version", "Print version information and exit.").Bool()
	listenAddressF = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9216").String()
	metricsPathF   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	collectDatabaseF   = kingpin.Flag("collect.database", "Enable collection of Database metrics").Bool()
	collectCollectionF = kingpin.Flag("collect.collection", "Enable collection of Collection metrics").Bool()
	collectTopF        = kingpin.Flag("collect.topmetrics", "Enable collection of table top metrics").Bool()
	collectIndexUsageF = kingpin.Flag("collect.indexusage", "Enable collection of per index usage stats").Bool()

	uriF = kingpin.Flag("mongodb.uri", "MongoDB URI, format").
		PlaceHolder("[mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]").
		Default("mongodb://localhost:27017").
		Envar("MONGODB_URI").
		String()

	tlsF     = kingpin.Flag("mongodb.tls", "Enable tls connection with mongo server").Bool()
	tlsCertF = kingpin.Flag("mongodb.tls-cert", "Path to PEM file that contains the certificate (and optionally also the decrypted private key in PEM format).\n"+
		"    \tThis should include the whole certificate chain.\n"+
		"    \tIf provided: The connection will be opened via TLS to the MongoDB server.").Default("").String()
	tlsPrivateKeyF = kingpin.Flag("mongodb.tls-private-key", "Path to PEM file that contains the decrypted private key (if not contained in mongodb.tls-cert file).").Default("").String()
	tlsCAF         = kingpin.Flag("mongodb.tls-ca", "Path to PEM file that contains the CAs that are trusted for server connections.\n"+
		"    \tIf provided: MongoDB servers connecting to should present a certificate signed by one of this CAs.\n"+
		"    \tIf not provided: System default CAs are used.").Default("").String()
	tlsDisableHostnameValidationF = kingpin.Flag("mongodb.tls-disable-hostname-validation", "Disable hostname validation for server connection.").Bool()
	maxConnectionsF               = kingpin.Flag("mongodb.max-connections", "Max number of pooled connections to the database.").Default("1").Int()
	testF                         = kingpin.Flag("test", "Check MongoDB connection, print buildInfo() information and exit.").Bool()

	socketTimeoutF = kingpin.Flag("mongodb.socket-timeout", "Amount of time to wait for a non-responding socket to the database before it is forcefully closed.\n"+
		"    \tValid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'.").Default("3s").Duration()
	syncTimeoutF = kingpin.Flag("mongodb.sync-timeout", "Amount of time an operation with this session will wait before returning an error in case\n"+
		"    \ta connection to a usable server can't be established.\n"+
		"    \tValid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'.").Default("1m").Duration()

	// FIXME currently ignored
	// enabledGroupsFlag = flag.String("groups.enabled", "asserts,durability,background_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,op_counters_repl,memory,locks,metrics", "Comma-separated list of groups to use, for more info see: docs.mongodb.org/manual/reference/command/serverStatus/")
	enabledGroupsFlag = kingpin.Flag("groups.enabled", "Currently ignored").Default("").String()
)

func main() {
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.Help = fmt.Sprintf("%s %s exports various MongoDB metrics in Prometheus format.\n", os.Args[0], version.Version)
	kingpin.Parse()

	if *testF {
		buildInfo, err := shared.TestConnection(
			shared.MongoSessionOpts{
				URI:                   *uriF,
				TLSConnection:         *tlsF,
				TLSCertificateFile:    *tlsCertF,
				TLSPrivateKeyFile:     *tlsPrivateKeyF,
				TLSCaFile:             *tlsCAF,
				TLSHostnameValidation: !(*tlsDisableHostnameValidationF),
			},
		)
		if err != nil {
			log.Errorf("Can't connect to MongoDB: %s", err)
			os.Exit(1)
		}
		fmt.Println(string(buildInfo))
		os.Exit(0)
	}
	if *versionF {
		fmt.Println(version.Print(program))
		os.Exit(0)
	}

	mongodbCollector := collector.NewMongodbCollector(&collector.MongodbCollectorOpts{
		URI:                      *uriF,
		TLSConnection:            *tlsF,
		TLSCertificateFile:       *tlsCertF,
		TLSPrivateKeyFile:        *tlsPrivateKeyF,
		TLSCaFile:                *tlsCAF,
		TLSHostnameValidation:    !(*tlsDisableHostnameValidationF),
		DBPoolLimit:              *maxConnectionsF,
		CollectDatabaseMetrics:   *collectDatabaseF,
		CollectCollectionMetrics: *collectCollectionF,
		CollectTopMetrics:        *collectTopF,
		CollectIndexUsageStats:   *collectIndexUsageF,
		SocketTimeout:            *socketTimeoutF,
		SyncTimeout:              *syncTimeoutF,
	})
	prometheus.MustRegister(mongodbCollector)

	exporter_shared.RunServer("MongoDB", *listenAddressF, *metricsPathF, promhttp.ContinueOnError)
}
