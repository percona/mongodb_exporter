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
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

// ServerMap stores http handlers for each host
type ServerMap map[string]http.Handler

// ServerOpts is the options for the main http handler
type ServerOpts struct {
	Path                   string
	MultiTargetPath        string
	OverallTargetPath      string
	WebListenAddress       string
	TLSConfigPath          string
	DisableDefaultRegistry bool
}

// RunWebServer runs the main web-server
func RunWebServer(opts *ServerOpts, exporters []*Exporter, exporterOpts *Opts, log *slog.Logger) {
	mux := http.NewServeMux()
	serverMap := buildServerMap(exporters, log)

	exportersCache := make(map[string]*Exporter)
	var cacheMutex sync.Mutex

	// Prefill cache with existing exporters
	for _, exp := range exporters {
		cacheMutex.Lock()
		cacheKey := exp.opts.URI
		exportersCache[cacheKey] = exp
		cacheMutex.Unlock()
	}

	mux.HandleFunc(opts.Path, func(w http.ResponseWriter, r *http.Request) {
		targetHost := r.URL.Query().Get("target")

		if targetHost == "" {
			// Serve local and cached exporter metrics
			if len(exporters) > 0 {
				exporters[0].Handler().ServeHTTP(w, r)
				return
			}

			// No local exporters, try to serve first cached exporter
			cacheMutex.Lock()
			defer cacheMutex.Unlock()
			for _, exp := range exportersCache {
				exp.Handler().ServeHTTP(w, r)
				return
			}

			reg := prometheus.NewRegistry()
			h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		multiTargetHandler(serverMap, exporterOpts, exportersCache, &cacheMutex, log).ServeHTTP(w, r)
	})

	mux.HandleFunc(opts.MultiTargetPath, multiTargetHandler(serverMap, exporterOpts, exportersCache, &cacheMutex, log))
	mux.HandleFunc(opts.OverallTargetPath, OverallTargetsHandler(exporters, log))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
            <head><title>MongoDB Exporter</title></head>
            <body>
            <h1>MongoDB Exporter</h1>
            <p><a href='/metrics'>Metrics</a></p>
            </body>
            </html>`))
		if err != nil {
			log.Error("error writing response", "error", err)
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
	if err := web.ListenAndServe(server, flags, log); err != nil {
		log.Error("error starting server", "error", err)
		os.Exit(1)
	}
}

// multiTargetHandler returns a handler that scrapes metrics from a target specified by the 'target' query parameter.
// It validates the URI and caches dynamic exporters by target.
func multiTargetHandler(serverMap ServerMap, exporterOpts *Opts, exportersCache map[string]*Exporter, cacheMutex *sync.Mutex, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetHost := r.URL.Query().Get("target")
		if targetHost == "" {
			logger.Warn("Missing target parameter")
			http.Error(w, "Missing target parameter", http.StatusBadRequest)
			return
		}

		parsed, err := url.Parse(targetHost)
		if err != nil {
			logger.Warn("Invalid target parameter", "target", targetHost, "error", err)
			http.Error(w, "Invalid target parameter", http.StatusBadRequest)
			return
		}

		fullURI := targetHost
		if parsed.User == nil && exporterOpts.User != "" {
			fullURI = BuildURI(targetHost, exporterOpts.User, exporterOpts.Password)
		}

		uri, err := url.Parse(fullURI)
		if err != nil {
			logger.Warn("Invalid full URI", "target", targetHost, "error", err)
			http.Error(w, "Invalid target parameter", http.StatusBadRequest)
			return
		}

		if handler, ok := serverMap[uri.Host]; ok {
			logger.Debug("Serving from static serverMap", "host", uri.Host)
			handler.ServeHTTP(w, r)
			return
		}

		cacheMutex.Lock()
		exp, ok := exportersCache[fullURI]
		cacheMutex.Unlock()

		if !ok {
			logger.Info("Creating new exporter for target", "target", targetHost)
			opts := *exporterOpts
			opts.URI = fullURI
			opts.Logger = logger

			exp = New(&opts)

			cacheMutex.Lock()
			exportersCache[fullURI] = exp
			cacheMutex.Unlock()
		} else {
			logger.Debug("Serving from cache", "target", targetHost)
		}

		exp.Handler().ServeHTTP(w, r)
	}
}

// OverallTargetsHandler is a handler to scrape all the targets in one request.
// Adds instance label to each metric.
func OverallTargetsHandler(exporters []*Exporter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seconds, err := strconv.Atoi(r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"))
		// To support older ones vmagents.
		if err != nil {
			seconds = 10
			logger.Debug("Can't get X-Prometheus-Scrape-Timeout-Seconds header, using default value 10")
		}

		var gatherers prometheus.Gatherers
		gatherers = append(gatherers, prometheus.DefaultGatherer)

		filters := r.URL.Query()["collect[]"]

		for _, e := range exporters {
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds-e.opts.TimeoutOffset)*time.Second)
			defer cancel()

			requestOpts := GetRequestOpts(filters, e.opts)

			client, err := e.getClient(ctx)
			if err != nil {
				e.logger.Error("Cannot connect to MongoDB", "error", err)
			}

			// Close client after usage.
			if !e.opts.GlobalConnPool {
				defer func() {
					if client != nil {
						if err := client.Disconnect(ctx); err != nil {
							logger.Error("Cannot disconnect client", "error", err)
						}
					}
				}()
			}

			var registry *prometheus.Registry
			var ti *topologyInfo
			if client != nil {
				// Topology can change between requests, so we need to get it every time.
				ti = newTopologyInfo(ctx, client, e.logger)
				registry = e.makeRegistry(ctx, client, ti, requestOpts)
			} else {
				registry = prometheus.NewRegistry()
				gc := newGeneralCollector(ctx, client, "", e.opts.Logger)
				registry.MustRegister(gc)
			}

			hostlabels := prometheus.Labels{}
			if e.opts.NodeName != "" {
				hostlabels["instance"] = e.opts.NodeName
			}

			gw := NewGathererWrapper(registry, hostlabels)
			gatherers = append(gatherers, gw)
		}

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog:      newHTTPErrorLogger(logger),
		})

		h.ServeHTTP(w, r)
	}
}

func buildServerMap(exporters []*Exporter, log *slog.Logger) ServerMap {
	servers := make(ServerMap, len(exporters))
	for _, e := range exporters {
		if parsedURL, err := url.Parse(e.opts.URI); err == nil {
			servers[parsedURL.Host] = e.Handler()
		} else {
			log.Error("Unable to parse provided address as url", "address", e.opts.URI, "error", err)
		}
	}

	return servers
}
