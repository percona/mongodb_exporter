package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Percona-Lab/prometheus_mongodb_exporter/collector"
	"github.com/Percona-Lab/prometheus_mongodb_exporter/shared"

	"github.com/prometheus/client_golang/prometheus"
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

	mongodbURIFlag    = flag.String("mongodb.uri", mongodbDefaultUri(), "Mongodb URI, format: [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]")
	enabledGroupsFlag = flag.String("groups.enabled", "asserts,durability,background_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,op_counters_repl,memory,locks,metrics", "Comma-separated list of groups to use, for more info see: docs.mongodb.org/manual/reference/command/serverStatus/")
)

func printVersion() {
	fmt.Printf("mongodb_exporter version: %s, git commit hash: %s\n", version, versionGitCommit)
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
	var authUser, authPass string
	httpAuth := os.Getenv("HTTP_AUTH")
	if httpAuth != "" {
		data := strings.SplitN(httpAuth, ":", 2)
		if len(data) != 2 || data[0] == "" || data[1] == "" {
			panic("HTTP_AUTH should be formatted as user:password")
		}
		authUser = data[0]
		authPass = data[1]
		fmt.Println("HTTP basic authentication is enabled")
	}

	handler := prometheus.Handler()
	if authUser != "" &&  authPass != "" {
		handler = &basicAuthHandler{
			handler:  handler.ServeHTTP,
			user:     authUser,
			password: authPass,
		}
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

	http.Handle(*metricsPathFlag, handler)
	fmt.Printf("Listening on %s\n", *listenAddressFlag)
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

	if *doPrintVersion {
		printVersion()
		os.Exit(0)
	}

	shared.ParseEnabledGroups(*enabledGroupsFlag)

	fmt.Println("### Warning: the exporter is in beta/experimental state and field names are very\n### likely to change in the future and features may change or get removed!\n### See: https://github.com/Percona-Lab/prometheus_mongodb_exporter for updates")

	startWebServer()
}
