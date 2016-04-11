package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/dcu/mongodb_exporter/collector"
	"github.com/dcu/mongodb_exporter/shared"

	"github.com/prometheus/client_golang/prometheus"
)

func mongodbDefaultUri() string {
	if u := os.Getenv("MONGODB_URL"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

var (
	listenAddressFlag = flag.String("web.listen-address", ":9001", "Address on which to expose metrics and web interface.")
	metricsPathFlag   = flag.String("web.metrics-path", "/metrics", "Path under which to expose metrics.")

	mongodbURIFlag    = flag.String("mongodb.uri", mongodbDefaultUri(), "Mongodb URI, format: [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]")
	enabledGroupsFlag = flag.String("groups.enabled", "asserts,durability,background_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,op_counters_repl,memory,locks,metrics", "Comma-separated list of groups to use, for more info see: docs.mongodb.org/manual/reference/command/serverStatus/")
	authUserFlag      = flag.String("auth.user", "", "Username for basic auth.")
	authPassFlag      = flag.String("auth.pass", "", "Password for basic auth.")
)

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

func hasUserAndPassword() bool {
	return *authUserFlag != "" && *authPassFlag != ""
}

func prometheusHandler() http.Handler {
	handler := prometheus.Handler()
	if hasUserAndPassword() {
		handler = &basicAuthHandler{
			handler:  prometheus.Handler().ServeHTTP,
			user:     *authUserFlag,
			password: *authPassFlag,
		}
	}

	return handler
}

func startWebServer() {
	fmt.Printf("Listening on %s\n", *listenAddressFlag)
	handler := prometheusHandler()

	registerCollector()

	http.Handle(*metricsPathFlag, handler)
	err := http.ListenAndServe(*listenAddressFlag, nil)

	if err != nil {
		panic(err)
	}
}

func registerCollector() {
	mongodbCollector := collector.NewMongodbCollector(collector.MongodbCollectorOpts{
		URI: *mongodbURIFlag,
	})
	prometheus.MustRegister(mongodbCollector)
}

func main() {
	flag.Parse()
	shared.ParseEnabledGroups(*enabledGroupsFlag)

	startWebServer()
}
