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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"

	"github.com/percona/mongodb_exporter/collector"
	"github.com/percona/mongodb_exporter/shared"
)

const (
	program = "mongodb_exporter"
)

func defaultMongoDBURL() string {
	if u := os.Getenv("MONGODB_URL"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

var (
	versionF       = flag.Bool("version", false, "Print version information and exit.")
	listenAddressF = flag.String("web.listen-address", ":9216", "Address to listen on for web interface and telemetry.")
	metricsPathF   = flag.String("web.metrics-path", "/metrics", "Path under which to expose metrics.")

	collectDatabaseF   = flag.Bool("collect.database", false, "Enable collection of Database metrics")
	collectCollectionF = flag.Bool("collect.collection", false, "Enable collection of Collection metrics")
	collectTopF        = flag.Bool("collect.topmetrics", false, "Enable collection of table top metrics")
	collectIndexUsageF = flag.Bool("collect.indexusage", false, "Enable collection of per index usage stats")

	uriF     = flag.String("mongodb.uri", defaultMongoDBURL(), "MongoDB URI, format: [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]")
	tlsF     = flag.Bool("mongodb.tls", false, "Enable tls connection with mongo server")
	tlsCertF = flag.String("mongodb.tls-cert", "", "Path to PEM file that contains the certificate (and optionally also the decrypted private key in PEM format).\n"+
		"    \tThis should include the whole certificate chain.\n"+
		"    \tIf provided: The connection will be opened via TLS to the MongoDB server.")
	tlsPrivateKeyF = flag.String("mongodb.tls-private-key", "", "Path to PEM file that contains the decrypted private key (if not contained in mongodb.tls-cert file).")
	tlsCAF         = flag.String("mongodb.tls-ca", "", "Path to PEM file that contains the CAs that are trusted for server connections.\n"+
		"    \tIf provided: MongoDB servers connecting to should present a certificate signed by one of this CAs.\n"+
		"    \tIf not provided: System default CAs are used.")
	tlsDisableHostnameValidationF = flag.Bool("mongodb.tls-disable-hostname-validation", false, "Disable hostname validation for server connection.")
	maxConnectionsF               = flag.Int("mongodb.max-connections", 1, "Max number of pooled connections to the database.")
	testF                         = flag.Bool("test", false, "Check MongoDB connection, print buildInfo() information and exit.")

	socketTimeoutF = flag.Duration("mongodb.socket-timeout", 3*time.Second, "Amount of time to wait for a non-responding socket to the database before it is forcefully closed.\n"+
		"    \tValid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'.")
	syncTimeoutF = flag.Duration("mongodb.sync-timeout", time.Minute, "Amount of time an operation with this session will wait before returning an error in case\n"+
		"    \ta connection to a usable server can't be established.\n"+
		"    \tValid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'.")

	// FIXME currently ignored
	// enabledGroupsFlag = flag.String("groups.enabled", "asserts,durability,background_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,op_counters_repl,memory,locks,metrics", "Comma-separated list of groups to use, for more info see: docs.mongodb.org/manual/reference/command/serverStatus/")
	enabledGroupsFlag = flag.String("groups.enabled", "", "Currently ignored")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s %s exports various MongoDB metrics in Prometheus format.\n", os.Args[0], version.Version)
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	uri := os.Getenv("MONGODB_URI")
	if uri != "" {
		uriF = &uri
	}

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
