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
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

var (
	errNoExporters          = errors.New("no exporters were built; specify --mongodb.uri, MONGODB_URI, or a dynamic target config")
	errTargetRequired       = errors.New("target is required")
	errInvalidTarget        = errors.New("invalid MongoDB target")
	errUnsupportedTargetURL = errors.New("target must contain only one MongoDB host and optional port")
)

// ServerMap stores http handlers for each host.
type ServerMap map[string]http.Handler

// DynamicTargetFactory builds a handler for a target supplied at scrape time.
type DynamicTargetFactory func(target, authModule string) (http.Handler, error)

// ServerOpts is the options for the main http handler
type ServerOpts struct {
	Path                   string
	MultiTargetPath        string
	DynamicTargetPath      string
	OverallTargetPath      string
	WebListenAddress       string
	TLSConfigPath          string
	DisableDefaultRegistry bool
	DynamicTargetFactory   DynamicTargetFactory
}

// RunWebServer runs the main web-server
func RunWebServer(opts *ServerOpts, exporters []*Exporter, log *slog.Logger) {
	handler, err := newWebHandler(opts, exporters, log)
	if err != nil {
		panic(err)
	}

	server := &http.Server{
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           handler,
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

func newWebHandler(opts *ServerOpts, exporters []*Exporter, log *slog.Logger) (http.Handler, error) {
	if len(exporters) == 0 && opts.DynamicTargetFactory == nil {
		return nil, errNoExporters
	}

	mux := http.NewServeMux()
	serverMap := buildServerMap(exporters, log)
	switch {
	case len(exporters) > 0:
		mux.Handle(opts.Path, exporters[0].Handler())
	case opts.DisableDefaultRegistry:
		mux.Handle(opts.Path, promhttp.HandlerFor(prometheus.NewRegistry(), promhttp.HandlerOpts{}))
	default:
		mux.Handle(opts.Path, promhttp.Handler())
	}

	targetHandler := multiTargetHandler(serverMap, opts.DynamicTargetFactory)
	if opts.MultiTargetPath != "" {
		mux.HandleFunc(opts.MultiTargetPath, targetHandler)
	}
	if opts.DynamicTargetPath != "" && opts.DynamicTargetPath != opts.MultiTargetPath {
		mux.HandleFunc(opts.DynamicTargetPath, targetHandler)
	}
	if opts.OverallTargetPath != "" {
		mux.HandleFunc(opts.OverallTargetPath, OverallTargetsHandler(exporters, log))
	}

	if opts.Path != "/" {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
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
	}

	return mux, nil
}

func multiTargetHandler(serverMap ServerMap, factory DynamicTargetFactory) http.HandlerFunc {
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

		if factory != nil {
			target, err := parseDynamicTarget(targetHost)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}

			handler, err := factory(target, r.URL.Query().Get("auth_module"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}
			if handler == nil {
				http.Error(w, "unable to build target handler", http.StatusInternalServerError)

				return
			}

			handler.ServeHTTP(w, r)

			return
		}

		http.Error(w, "unable to find target", http.StatusNotFound)
	}
}

func parseDynamicTarget(target string) (string, error) {
	if target == "" {
		return "", errTargetRequired
	}
	if !strings.HasPrefix(target, "mongodb://") {
		target = "mongodb://" + target
	}

	uri, err := url.Parse(target)
	if err != nil || uri.Host == "" {
		return "", errInvalidTarget
	}
	if uri.Scheme != "mongodb" || uri.User != nil || uri.Path != "" || uri.RawQuery != "" || uri.Fragment != "" || strings.Contains(uri.Host, ",") {
		return "", errUnsupportedTargetURL
	}

	return uri.Host, nil
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
