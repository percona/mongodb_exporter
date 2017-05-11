package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/percona/mongodb_exporter/collector"
	"github.com/percona/mongodb_exporter/shared"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

func mongodbDefaultUri() string {
	if u := os.Getenv("MONGODB_URL"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

var (
	version          string = "unknown"
	versionGitCommit string = "unknown"

	doPrintVersion    = flag.Bool("version", false, "Print version info and exit.")
	listenAddressFlag = flag.String("web.listen-address", ":9104", "Address on which to expose metrics and web interface.")
	metricsPathFlag   = flag.String("web.metrics-path", "/metrics", "Path under which to expose metrics.")
	webAuthFile       = flag.String("web.auth-file", "", "Path to YAML file with server_user, server_password options for http basic auth (overrides HTTP_AUTH env var).")
	sslCertFile       = flag.String("web.ssl-cert-file", "", "Path to SSL certificate file.")
	sslKeyFile        = flag.String("web.ssl-key-file", "", "Path to SSL key file.")
	mongodbURIFlag    = flag.String("mongodb.uri", mongodbDefaultUri(), "Mongodb URI, format: [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]")
	enabledGroupsFlag = flag.String("groups.enabled", "asserts,durability,background_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,op_counters_repl,memory,locks,metrics", "Comma-separated list of groups to use, for more info see: docs.mongodb.org/manual/reference/command/serverStatus/")
	mongodbTls        = flag.Bool("mongodb.tls", false, "Enable tls connection with mongo server")
	mongodbTlsCert    = flag.String("mongodb.tls-cert", "", "Path to PEM file that conains the certificate (and opionally also the private key in PEM format).\n"+
		"    \tThis should include the whole certificate chain.\n"+
		"    \tIf provided: The connection will be opened via TLS to the MongoDB server.")
	mongodbTlsPrivateKey = flag.String("mongodb.tls-private-key", "", "Path to PEM file that conains the private key (if not contained in mongodb.tls-cert file).")
	mongodbTlsCa         = flag.String("mongodb.tls-ca", "", "Path to PEM file that conains the CAs that are trused for server connections.\n"+
		"    \tIf provided: MongoDB servers connecting to should present a certificate signed by one of this CAs.\n"+
		"    \tIf not provided: System default CAs are used.")
	mongodbTlsDisableHostnameValidation = flag.Bool("mongodb.tls-disable-hostname-validation", false, "Do hostname validation for server connection.")
)

var landingPage = []byte(`<html>
<head><title>MongoDB exporter</title></head>
<body>
<h1>MongoDB exporter</h1>
<p><a href='` + *metricsPathFlag + `'>Metrics</a></p>
</body>
</html>
`)

func printVersion() {
	fmt.Printf("mongodb_exporter version: %s, git commit hash: %s\n", version, versionGitCommit)
}

type webAuth struct {
	User     string `yaml:"server_user,omitempty"`
	Password string `yaml:"server_password,omitempty"`
}

type basicAuthHandler struct {
	handler  http.HandlerFunc
	user     string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok || password != h.password || user != h.user {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
	return
}

func prometheusHandler() http.Handler {
	cfg := &webAuth{}
	httpAuth := os.Getenv("HTTP_AUTH")
	if *webAuthFile != "" {
		bytes, err := ioutil.ReadFile(*webAuthFile)
		if err != nil {
			panic(fmt.Sprintf("Cannot read auth file: %s", err))
		}
		if err := yaml.Unmarshal(bytes, cfg); err != nil {
			panic(fmt.Sprintf("Cannot parse auth file: %s", err))
		}
	} else if httpAuth != "" {
		data := strings.SplitN(httpAuth, ":", 2)
		if len(data) != 2 || data[0] == "" || data[1] == "" {
			panic("HTTP_AUTH should be formatted as user:password")
		}
		cfg.User = data[0]
		cfg.Password = data[1]
	}

	handler := prometheus.Handler()
	if cfg.User != "" && cfg.Password != "" {
		handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		fmt.Println("HTTP basic authentication is enabled")
	}

	return handler
}

func startWebServer() {
	printVersion()

	uri := os.Getenv("MONGODB_URI")
	if uri != "" {
		mongodbURIFlag = &uri
	}

	handler := prometheusHandler()

	registerCollector()

	if *sslCertFile != "" && *sslKeyFile == "" || *sslCertFile == "" && *sslKeyFile != "" {
		panic("One of the flags -web.ssl-cert or -web.ssl-key is missed to enable HTTPS/TLS")
	}
	ssl := false
	if *sslCertFile != "" && *sslKeyFile != "" {
		if _, err := os.Stat(*sslCertFile); os.IsNotExist(err) {
			panic(fmt.Sprintf("SSL certificate file does not exist: %s", *sslCertFile))
		}
		if _, err := os.Stat(*sslKeyFile); os.IsNotExist(err) {
			panic(fmt.Sprintf("SSL key file does not exist: %s", *sslKeyFile))
		}
		ssl = true
		fmt.Println("HTTPS/TLS is enabled")
	}

	fmt.Printf("Listening on %s\n", *listenAddressFlag)
	if ssl {
		// https
		mux := http.NewServeMux()
		mux.Handle(*metricsPathFlag, handler)
		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			w.Write(landingPage)
		})
		tlsCfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
		srv := &http.Server{
			Addr:         *listenAddressFlag,
			Handler:      mux,
			TLSConfig:    tlsCfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}
		panic(srv.ListenAndServeTLS(*sslCertFile, *sslKeyFile))
	} else {
		// http
		http.Handle(*metricsPathFlag, handler)
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(landingPage)
		})
		panic(http.ListenAndServe(*listenAddressFlag, nil))
	}
}

func registerCollector() {
	mongodbCollector := collector.NewMongodbCollector(collector.MongodbCollectorOpts{
		URI:                   *mongodbURIFlag,
		TLSConnection:         *mongodbTls,
		TLSCertificateFile:    *mongodbTlsCert,
		TLSPrivateKeyFile:     *mongodbTlsPrivateKey,
		TLSCaFile:             *mongodbTlsCa,
		TLSHostnameValidation: !(*mongodbTlsDisableHostnameValidation),
	})
	prometheus.MustRegister(mongodbCollector)
}

func main() {
	flag.Parse()

	if *doPrintVersion {
		printVersion()
		os.Exit(0)
	}

	shared.ParseEnabledGroups(*enabledGroupsFlag)

	fmt.Println("### Warning: the exporter is in beta/experimental state and field names are very\n### likely to change in the future and features may change or get removed!\n### See: https://github.com/percona/mongodb_exporter for updates")

	startWebServer()
}
