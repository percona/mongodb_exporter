package exporter

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/prometheus/common/promlog"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/sirupsen/logrus"
)

var ServerMap map[string]http.Handler

type ServerOpts struct {
	Path             string
	MultiTargetPath  string
	WebListenAddress string
	TLSConfigPath    string
}

func RunWebServer(opts *ServerOpts, exporters []*Exporter, log *logrus.Logger) {
	mux := http.DefaultServeMux

	if len(exporters) == 0 {
		panic("No exporters were builded. You must specify --mongodb.uri command argument or MONGODB_URI environment variable")
	}

	ServerMap = buildServerMap(exporters, log)

	defaultExporter := exporters[0]
	mux.Handle(opts.Path, defaultExporter.Handler())
	mux.HandleFunc(opts.MultiTargetPath, multiTargetHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
            <head><title>MongoDB Exporter</title></head>
            <body>
            <h1>MongoDB Exporter</h1>
            <p><a href='/metrics'>Metrics</a></p>
            </body>
            </html>`))
	})

	server := &http.Server{
		Addr:    opts.WebListenAddress,
		Handler: mux,
	}

	if err := web.ListenAndServe(server, opts.TLSConfigPath, promlog.New(&promlog.Config{})); err != nil {
		log.Errorf("error starting server: %v", err)
		os.Exit(1)
	}
}

func multiTargetHandler(w http.ResponseWriter, r *http.Request) {
	targetHost := r.URL.Query().Get("target")
	if targetHost != "" {
		if !strings.HasPrefix(targetHost, "mongodb://") {
			targetHost = "mongodb://" + targetHost
		}
		if uri, err := url.Parse(targetHost); err == nil {
			if e, ok := ServerMap[uri.Host]; ok {
				e.ServeHTTP(w, r)
				return
			}
		}
	}
	http.Error(w, "Unable to find target", http.StatusNotFound)
}

func buildServerMap(exporters []*Exporter, log *logrus.Logger) map[string]http.Handler {
	serverMap := make(map[string]http.Handler, len(exporters))
	for _, e := range exporters {
		if url, err := url.Parse(e.opts.URI); err == nil {
			serverMap[url.Host] = e.Handler()
		} else {
			log.Errorf("Unable to parse addr %s as url: %s", e.opts.URI, err)
		}
	}
	return serverMap
}
