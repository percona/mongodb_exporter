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
	"runtime"
	"strings"
	"testing"
	"time"

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
// MongoDB connections or goroutines across multiple scrape cycles.
//
// This test monitors both server-side connections and client-side goroutines
// to detect leaks. A leak in the PBM SDK's connect.MongoConnectWithOpts()
// function (where Ping() failure doesn't call Disconnect()) would cause
// goroutine growth even if server connections appear stable.
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

	// Allow GC and goroutines to stabilize before measuring
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	initialConns := getServerConnectionCount(ctx, t, client)
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial state: connections=%d, goroutines=%d", initialConns, initialGoroutines)

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

	// Allow time for cleanup and GC
	time.Sleep(1 * time.Second)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalConns := getServerConnectionCount(ctx, t, client)
	finalGoroutines := runtime.NumGoroutine()
	connGrowth := finalConns - initialConns
	goroutineGrowth := finalGoroutines - initialGoroutines

	t.Logf("Final state: connections=%d (growth=%d), goroutines=%d (growth=%d) over %d scrapes",
		finalConns, connGrowth, finalGoroutines, goroutineGrowth, scrapeCount)

	// Check for connection leaks
	// With a leak: expect ~3-4 connections per scrape (one per cluster member)
	// Without leak: should be near zero growth
	assert.Less(t, connGrowth, int64(scrapeCount),
		"Connection leak detected: server connections grew by %d over %d scrapes", connGrowth, scrapeCount)

	// Check for goroutine leaks (more sensitive indicator)
	// Each leaked mongo.Client creates ~24 goroutines for monitoring
	// Allow some variance for GC timing, but flag major growth
	maxGoroutineGrowth := scrapeCount * 5 // Allow 5 goroutines per scrape as buffer
	assert.Less(t, goroutineGrowth, maxGoroutineGrowth,
		"Goroutine leak detected: grew by %d over %d scrapes (max allowed: %d). "+
			"This may indicate MongoDB clients are not being properly disconnected.",
		goroutineGrowth, scrapeCount, maxGoroutineGrowth)
}

// TestMongoClientLeakOnPingFailure demonstrates the goroutine leak that occurs
// when mongo.Connect succeeds but Ping fails, and Disconnect is not called.
//
// This test reproduces the bug in PBM SDK's connect.MongoConnectWithOpts()
// where a failed Ping does not call Disconnect on the client, causing leaked
// goroutines and connections.
//
// The test uses a ReadPreference with a non-existent tag to force Ping to fail
// after the driver has already established monitoring connections.
//
// This test documents the leak behavior. It passes to show the leak exists.
// After the PBM SDK fix, this test can be modified to verify the fix.
//
//nolint:paralleltest
func TestMongoClientLeakOnPingFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	port, err := tu.PortForContainer("mongo-2-1")
	require.NoError(t, err)

	// Allow GC and goroutines to stabilize before measuring
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	const iterations = 5
	leakedClients := make([]*mongo.Client, 0, iterations)

	for i := 0; i < iterations; i++ {
		// Use a ReadPreference that can't be satisfied - this causes:
		// 1. Connect to succeed (driver starts monitoring goroutines)
		// 2. Ping to fail (no server matches the tag selector)
		opts := options.Client().
			ApplyURI(fmt.Sprintf("mongodb://admin:admin@127.0.0.1:%s/", port)).
			SetReadPreference(readpref.Nearest(readpref.WithTags("dc", "nonexistent"))).
			SetServerSelectionTimeout(300 * time.Millisecond)

		client, err := mongo.Connect(ctx, opts)
		require.NoError(t, err, "Connect should succeed")

		// Give the driver time to establish monitoring connections
		time.Sleep(100 * time.Millisecond)

		// Ping will fail because no server matches the read preference
		err = client.Ping(ctx, nil)
		require.Error(t, err, "Ping should fail due to unsatisfiable read preference")
		t.Logf("Iteration %d: Ping failed as expected: %v", i, err)

		// Simulate the bug: NOT calling Disconnect after Ping failure
		// This is what happens in connect.MongoConnectWithOpts when Ping fails
		leakedClients = append(leakedClients, client)
	}

	// Allow time for any cleanup and measure
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	goroutinesAfterLeak := runtime.NumGoroutine()
	leakGrowth := goroutinesAfterLeak - initialGoroutines

	t.Logf("After %d iterations without Disconnect: goroutines=%d (growth=%d)",
		iterations, goroutinesAfterLeak, leakGrowth)

	// Each mongo.Client creates ~24 goroutines for monitoring
	// With 5 iterations, we expect ~120 leaked goroutines
	expectedLeakPerClient := 20 // Conservative estimate
	expectedTotalLeak := iterations * expectedLeakPerClient

	// Document that the leak exists
	if leakGrowth >= expectedTotalLeak/2 {
		t.Logf("LEAK CONFIRMED: %d goroutines leaked over %d iterations (expected ~%d)",
			leakGrowth, iterations, expectedTotalLeak)
	}

	// Now properly disconnect all clients and verify cleanup works
	for _, client := range leakedClients {
		client.Disconnect(ctx) //nolint:errcheck
	}

	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	goroutinesAfterCleanup := runtime.NumGoroutine()
	cleanupGrowth := goroutinesAfterCleanup - initialGoroutines

	t.Logf("After Disconnect cleanup: goroutines=%d (growth from initial=%d)",
		goroutinesAfterCleanup, cleanupGrowth)

	// After proper cleanup, goroutine count should return to near initial
	// This verifies that Disconnect() properly cleans up resources
	assert.Less(t, cleanupGrowth, 10,
		"Goroutines should return to near initial after Disconnect, but grew by %d", cleanupGrowth)

	// Document the leak - this assertion verifies the bug exists
	// When NOT calling Disconnect after Ping failure, goroutines leak
	assert.Greater(t, leakGrowth, expectedTotalLeak/2,
		"Expected goroutine leak when Disconnect is not called after Ping failure. "+
			"Growth was %d, expected at least %d. "+
			"This documents the bug in PBM SDK's connect.MongoConnectWithOpts()",
		leakGrowth, expectedTotalLeak/2)
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
