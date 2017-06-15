package main

import (
	"crypto/subtle"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/yaml.v2"
)

const (
	program = "mongodb_exporter"
)

func mongodbDefaultUri() string {
	if u := os.Getenv("MONGODB_URL"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

var (
	versionF       = flag.Bool("version", false, "Print version information and exit.")
	listenAddressF = flag.String("web.listen-address", ":9216", "Address to listen on for web interface and telemetry.")
	metricsPathF   = flag.String("web.metrics-path", "/metrics", "Path under which to expose metrics.")
	authFileF      = flag.String("web.auth-file", "", "Path to YAML file with server_user, server_password options for http basic auth (overrides HTTP_AUTH env var).")
	sslCertFileF   = flag.String("web.ssl-cert-file", "", "Path to SSL certificate file.")
	sslKeyFileF    = flag.String("web.ssl-key-file", "", "Path to SSL key file.")

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
<p><a href='` + *metricsPathF + `'>Metrics</a></p>
</body>
</html>
`)

type webAuth struct {
	Username string `yaml:"server_user,omitempty"`
	Password string `yaml:"server_password,omitempty"`
}

type basicAuthHandler struct {
	webAuth
	handler http.HandlerFunc
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, _ := r.BasicAuth()
	usernameOk := subtle.ConstantTimeCompare([]byte(h.Username), []byte(username)) == 1
	passwordOk := subtle.ConstantTimeCompare([]byte(h.Password), []byte(password)) == 1
	if !usernameOk || !passwordOk {
		w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
}

// logger adapts log.Logger interface to promhttp.Logger interface.
// See https://github.com/prometheus/common/issues/86.
type logger struct {
	log.Logger
}

func (l logger) Println(v ...interface{}) {
	l.Errorln(v...)
}

// check interfaces
var (
	_ log.Logger      = logger{}
	_ promhttp.Logger = logger{}
)

func prometheusHandler() http.Handler {
	cfg := &webAuth{}
	httpAuth := os.Getenv("HTTP_AUTH")
	switch {
	case *authFileF != "":
		bytes, err := ioutil.ReadFile(*authFileF)
		if err != nil {
			log.Fatal("Cannot read auth file: ", err)
		}
		if err := yaml.Unmarshal(bytes, cfg); err != nil {
			log.Fatal("Cannot parse auth file: ", err)
		}
	case httpAuth != "":
		data := strings.SplitN(httpAuth, ":", 2)
		if len(data) != 2 || data[0] == "" || data[1] == "" {
			log.Fatal("HTTP_AUTH should be formatted as user:password")
		}
		cfg.Username = data[0]
		cfg.Password = data[1]
	}

	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog:      logger{log.Base()},
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
	if cfg.Username != "" && cfg.Password != "" {
		handler = &basicAuthHandler{webAuth: *cfg, handler: handler.ServeHTTP}
		log.Infoln("HTTP basic authentication is enabled")
	}

	return handler
}

func startWebServer() {
	uri := os.Getenv("MONGODB_URI")
	if uri != "" {
		mongodbURIFlag = &uri
	}

	handler := prometheusHandler()

	registerCollector()

	if (*sslCertFileF == "") != (*sslKeyFileF == "") {
		log.Fatal("One of the flags -web.ssl-cert-file or -web.ssl-key-file is missing to enable HTTPS/TLS")
	}
	ssl := false
	if *sslCertFileF != "" && *sslKeyFileF != "" {
		if _, err := os.Stat(*sslCertFileF); os.IsNotExist(err) {
			log.Fatal("SSL certificate file does not exist: ", *sslCertFileF)
		}
		if _, err := os.Stat(*sslKeyFileF); os.IsNotExist(err) {
			log.Fatal("SSL key file does not exist: ", *sslKeyFileF)
		}
		ssl = true
		log.Infoln("HTTPS/TLS is enabled")
	}

	log.Infoln("Listening on", *listenAddressF)
	if ssl {
		// https
		mux := http.NewServeMux()
		mux.Handle(*metricsPathF, handler)
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
			Addr:         *listenAddressF,
			Handler:      mux,
			TLSConfig:    tlsCfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		}
		log.Fatal(srv.ListenAndServeTLS(*sslCertFileF, *sslKeyFileF))
	} else {
		// http
		http.Handle(*metricsPathF, handler)
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(landingPage)
		})
		log.Fatal(http.ListenAndServe(*listenAddressF, nil))
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

	if *versionF {
		fmt.Println(version.Print(program))
		os.Exit(0)
	}

	shared.ParseEnabledGroups(*enabledGroupsFlag)

	fmt.Println("### Warning: the exporter is in beta/experimental state and field names are very\n### likely to change in the future and features may change or get removed!\n### See: https://github.com/percona/mongodb_exporter for updates")

	startWebServer()
}
