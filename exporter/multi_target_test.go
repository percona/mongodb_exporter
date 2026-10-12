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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestDynamicTarget(t *testing.T) {
	t.Parallel()

	var gotTarget, gotAuthModule string
	factory := func(target, authModule string) (http.Handler, error) {
		gotTarget = target
		gotAuthModule = authModule

		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}), nil
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe?target=mongo.example.com:3717&auth_module=cloud_mongo", nil)
	multiTargetHandler(nil, factory)(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "mongo.example.com:3717", gotTarget)
	assert.Equal(t, "cloud_mongo", gotAuthModule)
}

func TestDynamicTargetValidation(t *testing.T) {
	t.Parallel()

	factory := func(_, _ string) (http.Handler, error) {
		return nil, assert.AnError
	}
	tests := []string{
		"mongodb://user:password@mongo.example.com:3717",
		"mongodb://mongo.example.com:3717/admin",
		"mongodb://mongo.example.com:3717?tls=true",
	}

	for _, target := range tests {
		rr := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe?target="+url.QueryEscape(target), nil)
		multiTargetHandler(nil, factory)(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, target)
	}
}

func TestDynamicOnlyServer(t *testing.T) {
	t.Parallel()

	factory := func(target, authModule string) (http.Handler, error) {
		assert.Equal(t, "mongo.example.com:3717", target)
		assert.Equal(t, "cloud_mongo", authModule)

		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}), nil
	}
	opts := &ServerOpts{
		Path:                   "/metrics",
		MultiTargetPath:        "/scrape",
		DynamicTargetPath:      "/probe",
		DisableDefaultRegistry: true,
		DynamicTargetFactory:   factory,
	}

	handler, err := newWebHandler(opts, nil, promslog.New(&promslog.Config{}))
	require.NoError(t, err)

	metricsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(metricsRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusOK, metricsRecorder.Code)
	assert.Empty(t, strings.TrimSpace(metricsRecorder.Body.String()))

	probeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(probeRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe?target=mongo.example.com:3717&auth_module=cloud_mongo", nil))
	assert.Equal(t, http.StatusNoContent, probeRecorder.Code)
}

func TestServerRequiresStaticOrDynamicTarget(t *testing.T) {
	t.Parallel()

	_, err := newWebHandler(&ServerOpts{Path: "/metrics"}, nil, promslog.New(&promslog.Config{}))
	assert.ErrorContains(t, err, "no exporters were built")
}

func TestMultiTarget(t *testing.T) {
	hostname := "127.0.0.1"
	opts := []*Opts{
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017"))),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"))),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			URI:              fmt.Sprintf("mongodb://admin:admin@%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"))),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, "12345")),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
	}
	exporters := make([]*Exporter, len(opts))

	for i, opt := range opts {
		exporters[i] = New(opt)
	}
	log := promslog.New(&promslog.Config{})
	serverMap := buildServerMap(exporters, log)

	expected := []string{
		"mongodb_up{cluster_role=\"mongod\"} 1\n",
		"mongodb_up{cluster_role=\"mongod\"} 1\n",
		"mongodb_up{cluster_role=\"mongod\"} 1\n",
		"mongodb_up{cluster_role=\"\"} 0\n",
	}

	// Test all targets
	for sn, opt := range opts {
		assert.HTTPBodyContains(t, multiTargetHandler(serverMap, nil), "GET", "?target="+opt.URI, nil, expected[sn])
	}
}

func TestOverallHandler(t *testing.T) {
	t.Parallel()

	opts := []*Opts{
		{
			NodeName:         "standalone",
			URI:              fmt.Sprintf("mongodb://127.0.0.1:%s", tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017")),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			NodeName:         "s1",
			URI:              fmt.Sprintf("mongodb://127.0.0.1:%s", tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001")),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			NodeName:         "s2",
			URI:              fmt.Sprintf("mongodb://127.0.0.1:%s", tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004")),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			NodeName:         "s3",
			URI:              "mongodb://127.0.0.1:12345",
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
	}
	expected := []*regexp.Regexp{
		regexp.MustCompile(`mongodb_up{[^\}]*instance="standalone"[^\}]*} 1\n`),
		regexp.MustCompile(`mongodb_up{[^\}]*instance="s1"[^\}]*} 1\n`),
		regexp.MustCompile(`mongodb_up{[^\}]*instance="s2"[^\}]*} 1\n`),
		regexp.MustCompile(`mongodb_up{[^\}]*instance="s3"[^\}]*} 0\n`),
	}
	exporters := make([]*Exporter, len(opts))

	logger := promslog.New(&promslog.Config{})

	for i, opt := range opts {
		exporters[i] = New(opt)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	OverallTargetsHandler(exporters, logger)(rr, req)
	res := rr.Result()
	resBody, _ := io.ReadAll(res.Body)
	err := res.Body.Close()
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, res.StatusCode)

	for _, expected := range expected {
		assert.Regexp(t, expected, string(resBody))
	}
}
