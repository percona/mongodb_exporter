// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/prometheus/common/promlog"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/sirupsen/logrus"
)

// ServerMap stores http handlers for each host
type ServerMap map[string]http.Handler

// ServerOpts is the options for the main http handler
type ServerOpts struct {
	Path             string
	MultiTargetPath  string
	WebListenAddress string
	TLSConfigPath    string
}

// Runs the main web-server
func RunWebServer(opts *ServerOpts, exporters []*Exporter, log *logrus.Logger) {
	mux := http.DefaultServeMux

	if len(exporters) == 0 {
		panic("No exporters were built. You must specify --mongodb.uri command argument or MONGODB_URI environment variable")
	}

	serverMap := buildServerMap(exporters, log)

	defaultExporter := exporters[0]
	mux.Handle(opts.Path, defaultExporter.Handler())
	mux.HandleFunc(opts.MultiTargetPath, multiTargetHandler(serverMap))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
            <head><title>MongoDB Exporter</title></head>
            <body>
            <h1>MongoDB Exporter</h1>
            <p><a href='/metrics'>Metrics</a></p>
            </body>
            </html>`))
		if err != nil {
			log.Errorf("error writing response: %v", err)
		}
	})

	server := &http.Server{
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           mux,
	}
	flags := &web.FlagConfig{
		WebListenAddresses: &[]string{opts.WebListenAddress},
		WebConfigFile:      &opts.TLSConfigPath,
	}
	if err := web.ListenAndServe(server, flags, promlog.New(&promlog.Config{})); err != nil {
		log.Errorf("error starting server: %v", err)
		os.Exit(1)
	}
}

func multiTargetHandler(serverMap ServerMap) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetHost := r.URL.Query().Get("target")
		if targetHost != "" {
			if !strings.HasPrefix(targetHost, "mongodb://") {
				targetHost = "mongodb://" + targetHost
			}
			if uri, err := url.Parse(targetHost); err == nil {
				if e, ok := serverMap[uri.Host]; ok {
					e.ServeHTTP(w, r)
					return
				}
			}
		}
		http.Error(w, "Unable to find target", http.StatusNotFound)
	}
}

func buildServerMap(exporters []*Exporter, log *logrus.Logger) ServerMap {
	servers := make(ServerMap, len(exporters))
	for _, e := range exporters {
		if url, err := url.Parse(e.opts.URI); err == nil {
			servers[url.Host] = e.Handler()
		} else {
			log.Errorf("Unable to parse addr %s as url: %s", e.opts.URI, err)
		}
	}

	return servers
}
