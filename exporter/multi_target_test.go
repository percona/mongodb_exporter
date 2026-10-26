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
	"regexp"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"

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
		assert.HTTPBodyContains(t, multiTargetHandler(serverMap), "GET", fmt.Sprintf("?target=%s", opt.URI), nil, expected[sn])
	}
}

// overallTarget is one target /scrapeall is exercised against.
type overallTarget struct {
	nodeName string
	uri      string
	wantUp   bool
}

// overallTargets is three reachable nodes and one nothing listens on, so that one request
// covers both the serving path and the reporting of a target that is down.
func overallTargets() []overallTarget {
	return []overallTarget{
		{nodeName: "standalone", uri: "mongodb://127.0.0.1:" + tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017"), wantUp: true},
		{nodeName: "s1", uri: "mongodb://127.0.0.1:" + tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"), wantUp: true},
		{nodeName: "s2", uri: "mongodb://127.0.0.1:" + tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"), wantUp: true},
		{nodeName: "s3", uri: "mongodb://127.0.0.1:12345", wantUp: false},
	}
}

// scrapeAll runs the /scrapeall handler over exporters and returns the response body.
func scrapeAll(t *testing.T, exporters []*Exporter) string {
	t.Helper()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	OverallTargetsHandler(exporters, promslog.New(&promslog.Config{}))(rr, req)

	res := rr.Result()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	require.Equal(t, http.StatusOK, res.StatusCode)

	return string(body)
}

// assertOverallUp checks the mongodb_up line /scrapeall produced for each target.
func assertOverallUp(t *testing.T, body string, targets []overallTarget) {
	t.Helper()

	for _, target := range targets {
		up := "0"
		if target.wantUp {
			up = "1"
		}

		want := regexp.MustCompile(fmt.Sprintf(`mongodb_up\{[^}]*instance="%s"[^}]*\} %s\n`, target.nodeName, up))
		assert.Regexp(t, want, body, "mongodb_up for %s", target.nodeName)
	}
}

// /scrapeall serves every target with the connection pool on, which is the default, and with
// it off. The pooled half is what the default flip made the common case: one client per target
// outlives the request that built it, and the next request reuses it rather than connecting
// again -- which is also the half where a request holds a client other scrapes can evict, since
// the handler takes every client up front and only then collects.
func TestOverallHandler(t *testing.T) {
	t.Parallel()

	for _, pool := range []bool{true, false} {
		t.Run(fmt.Sprintf("pool=%v", pool), func(t *testing.T) {
			t.Parallel()

			targets := overallTargets()
			exporters := make([]*Exporter, len(targets))

			for i, target := range targets {
				exporters[i] = New(&Opts{
					NodeName:         target.nodeName,
					URI:              target.uri,
					DirectConnect:    true,
					ConnectTimeoutMS: 1000,
					GlobalConnPool:   pool,
				})
			}
			// Nothing in the pooled path ever disconnects a client, so the test has to.
			t.Cleanup(func() {
				for _, e := range exporters {
					if client := cachedClient(e); client != nil {
						_ = client.Disconnect(context.Background())
					}
				}
			})

			assertOverallUp(t, scrapeAll(t, exporters), targets)

			// A reachable target leaves a client behind when pooling, and never otherwise.
			cached := make([]*mongo.Client, len(targets))
			for i, target := range targets {
				cached[i] = cachedClient(exporters[i])
				if pool && target.wantUp {
					require.NotNil(t, cached[i], "%s: the scrape left no client in the pool", target.nodeName)
				} else {
					require.Nil(t, cached[i], "%s: a client was cached that should not have been", target.nodeName)
				}
			}

			// The second request reports the same thing off the same clients: still up, and the
			// pooled ones neither replaced nor disconnected by the request that used them.
			assertOverallUp(t, scrapeAll(t, exporters), targets)

			for i, target := range targets {
				if pool && target.wantUp {
					assert.Same(t, cached[i], cachedClient(exporters[i]),
						"%s: the second scrape replaced the pooled client", target.nodeName)
				}
			}
		})
	}
}
