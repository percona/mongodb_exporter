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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

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
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"))),
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
	log := logrus.New()
	serverMap := buildServerMap(exporters, log)

	expected := []string{
		"mongodb_up 1\n",
		"mongodb_up 1\n",
		"mongodb_up 1\n",
		"mongodb_up 0\n",
	}

	// Test all targets
	for sn, opt := range opts {
		assert.HTTPBodyContains(t, multiTargetHandler(serverMap), "GET", fmt.Sprintf("?target=%s", opt.URI), nil, expected[sn])
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

	logger := logrus.New()

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
