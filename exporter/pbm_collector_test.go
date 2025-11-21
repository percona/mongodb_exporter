// mongodb_exporter
// Copyright (C) 2024 Percona LLC
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
	"strings"
	"testing"
	"time"

	pbmconnect "github.com/percona/percona-backup-mongodb/pbm/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:paralleltest
func TestPBMCollector(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	port, err := tu.PortForContainer("mongo-2-1")
	require.NoError(t, err)
	client := tu.TestClient(ctx, port, t)
	mongoURI := "mongodb://admin:admin@127.0.0.1:17006/?connectTimeoutMS=1000&directConnection=true&serverSelectionTimeoutMS=1000" //nolint:gosec

	c := newPbmCollector(ctx, client, mongoURI, promslog.New(&promslog.Config{}))

	t.Run("pbm configured metric", func(t *testing.T) {
		filter := []string{
			"mongodb_pbm_cluster_backup_configured",
		}
		expected := strings.NewReader(`
		# HELP mongodb_pbm_cluster_backup_configured PBM backups are configured for the cluster
		# TYPE mongodb_pbm_cluster_backup_configured gauge
		mongodb_pbm_cluster_backup_configured 1` + "\n")
		err = testutil.CollectAndCompare(c, expected, filter...)
		assert.NoError(t, err)
	})

	t.Run("pbm agent status metric", func(t *testing.T) {
		filter := []string{
			"mongodb_pbm_agent_status",
		}
		expectedLength := 4 // we expect 4 metrics for each member of the RS (1 primary, 2 secondaries, 1 arbiter).
		count := testutil.CollectAndCount(c, filter...)
		assert.Equal(t, expectedLength, count, "PBM metrics are missing")
	})
}

// TestPBMCollectorConnectionLeak verifies that the PBM collector does not leak
// MongoDB connections across multiple scrape cycles.
//
//nolint:paralleltest
func TestPBMCollectorConnectionLeak(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	port, err := tu.PortForContainer("mongo-2-1")
	require.NoError(t, err)

	mongoURI := fmt.Sprintf(
		"mongodb://admin:admin@127.0.0.1:%s/?connectTimeoutMS=500&serverSelectionTimeoutMS=500",
		port,
	) //nolint:gosec

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Disconnect(ctx) //nolint:errcheck
	})
	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	// Allow connections to stabilize
	time.Sleep(100 * time.Millisecond)

	initialConns := getServerConnectionCount(ctx, t, client)
	t.Logf("Initial connections: %d", initialConns)

	c := newPbmCollector(ctx, client, mongoURI, promslog.New(&promslog.Config{}))

	// Run multiple collection cycles
	const scrapeCount = 10
	for i := 0; i < scrapeCount; i++ {
		ch := make(chan prometheus.Metric, 100)
		c.Collect(ch)
		close(ch)
		for range ch {
		}
	}

	// Allow time for cleanup
	time.Sleep(1 * time.Second)

	finalConns := getServerConnectionCount(ctx, t, client)
	connGrowth := finalConns - initialConns

	t.Logf("Final connections: %d (growth=%d over %d scrapes)", finalConns, connGrowth, scrapeCount)

	// With a leak: expect ~3-4 connections per scrape (one per cluster member)
	// Without leak: should be near zero growth
	assert.Less(t, connGrowth, int64(scrapeCount),
		"Connection leak detected: server connections grew by %d over %d scrapes", connGrowth, scrapeCount)
}

// TestPBMSDKConnectionLeakOnPingFailure tests the PBM SDK's connect.MongoConnect
// for connection leaks when Ping fails after Connect succeeds.
//
// The bug in PBM SDK's connect.MongoConnectWithOpts():
//   - mongo.Connect() succeeds (establishes connections)
//   - Ping() fails (e.g., unreachable server, wrong replica set)
//   - Connection is NOT disconnected -> connections leak
//
// This test uses an unsatisfiable ReadPreference to trigger Ping failure
// after the driver has established connections.
//
//nolint:paralleltest
func TestPBMSDKConnectionLeakOnPingFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	port, err := tu.PortForContainer("mongo-2-1")
	require.NoError(t, err)

	// Create a monitoring client to check server connection count
	monitorURI := fmt.Sprintf("mongodb://admin:admin@127.0.0.1:%s/", port) //nolint:gosec
	monitorClient, err := mongo.Connect(ctx, options.Client().ApplyURI(monitorURI))
	require.NoError(t, err)
	t.Cleanup(func() {
		monitorClient.Disconnect(ctx) //nolint:errcheck
	})
	err = monitorClient.Ping(ctx, nil)
	require.NoError(t, err)

	// Allow connections to stabilize
	time.Sleep(100 * time.Millisecond)

	initialConns := getServerConnectionCount(ctx, t, monitorClient)
	t.Logf("Initial connections: %d", initialConns)

	const iterations = 5

	for i := 0; i < iterations; i++ {
		// Use a ReadPreference that cannot be satisfied to trigger the leak:
		// 1. mongo.Connect() succeeds and establishes connections
		// 2. Ping() fails because no server matches the tag selector
		// 3. Without proper cleanup, connections are leaked
		_, err := pbmconnect.MongoConnect(ctx,
			monitorURI,
			pbmconnect.AppName("leak-test"),
			func(opts *options.ClientOptions) error {
				opts.SetReadPreference(readpref.Nearest(readpref.WithTags("dc", "nonexistent")))
				opts.SetServerSelectionTimeout(300 * time.Millisecond)
				return nil
			},
		)
		require.Error(t, err, "MongoConnect should fail due to unsatisfiable ReadPreference")
		t.Logf("Iteration %d: MongoConnect failed as expected", i)
	}

	// Allow time for any cleanup
	time.Sleep(500 * time.Millisecond)

	finalConns := getServerConnectionCount(ctx, t, monitorClient)
	connGrowth := finalConns - initialConns

	t.Logf("Final connections: %d (growth=%d over %d iterations)", finalConns, connGrowth, iterations)

	// Each leaked connection attempt should leave connections open
	// If there's a leak, we'd see growth proportional to iterations
	// Without leak, growth should be zero or minimal
	leakThreshold := int64(iterations * 2) // Allow some buffer

	if connGrowth >= leakThreshold {
		t.Logf("LEAK DETECTED: %d connections leaked over %d iterations", connGrowth, iterations)
		t.Logf("This confirms the bug in PBM SDK's connect.MongoConnectWithOpts()")
	}

	// This test documents the leak. Once the PBM SDK is fixed, flip this assertion:
	// assert.Less(t, connGrowth, leakThreshold, "Connection leak in PBM SDK")
	assert.GreaterOrEqual(t, connGrowth, leakThreshold,
		"Expected connection leak when Ping fails. Growth was %d, expected at least %d. "+
			"If this fails, the PBM SDK may have been fixed!",
		connGrowth, leakThreshold)
}

// getServerConnectionCount returns the current number of connections to the MongoDB server.
func getServerConnectionCount(ctx context.Context, t *testing.T, client *mongo.Client) int64 {
	t.Helper()

	var result bson.M
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&result)
	require.NoError(t, err)

	connections, ok := result["connections"].(bson.M)
	require.True(t, ok, "serverStatus should contain connections field")

	current, ok := connections["current"].(int32)
	if ok {
		return int64(current)
	}

	// Try int64 in case MongoDB returns it differently
	current64, ok := connections["current"].(int64)
	require.True(t, ok, "connections.current should be numeric")

	return current64
}
