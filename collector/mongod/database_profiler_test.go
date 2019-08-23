package mongod

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/shared"
	"github.com/percona/mongodb_exporter/testutils"
)

func TestGetDatabaseProfilerStatsDecodesFine(t *testing.T) {
	// setup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := testutils.MustGetConnectedReplSetClient(ctx, t)
	defer client.Disconnect(ctx)

	// enable profiling and run a query
	db := client.Database("test-profile")
	assert.NotNil(t, db)
	ok := db.RunCommand(ctx, bson.D{{"profile", 2}})
	assert.NoErrorf(t, ok.Err(), "failed to enable profiling")
	coll := db.Collection("test")
	assert.NotNil(t, coll)
	_, err := coll.InsertOne(ctx, bson.M{})
	assert.NoErrorf(t, err, "failed to run a profiled find")

	// run
	loopback := int64(10) // seconds.
	threshold := int64(0) // milliseconds.
	stats := GetDatabaseProfilerStats(client, loopback, threshold)

	// test
	assert.NotNil(t, stats)
	assert.Truef(t, len(stats.Members) >= 1, "expected at least one slow query")
}

func TestGetDatabaseProfilerStatsMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping functional test")
	}

	// setup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := testutils.MustGetConnectedReplSetClient(ctx, t)
	defer client.Disconnect(ctx)

	// enable profiling and run a query
	db := client.Database("test-profile")
	assert.NotNil(t, db)
	ok := db.RunCommand(ctx, bson.D{{"profile", 2}})
	assert.NoErrorf(t, ok.Err(), "failed to enable profiling")
	coll := db.Collection("test")
	assert.NotNil(t, coll)
	_, err := coll.InsertOne(ctx, bson.M{})
	assert.NoErrorf(t, err, "failed to run a profiled find")

	// run
	loopback := int64(10) // seconds.
	threshold := int64(1) // milliseconds.
	stats := GetDatabaseProfilerStats(client, loopback, threshold)

	// test
	assert.NotNil(t, stats)
	metricCh := make(chan prometheus.Metric)
	go func() {
		stats.Export(metricCh)
		close(metricCh)
	}()

	var metricsCount int
	for range metricCh {
		metricsCount++
	}
	assert.Truef(t, metricsCount >= 1, "expected at least one slow query metric")
}

func TestGetDatabaseCurrentOpStatsDecodesFine(t *testing.T) {
	// setup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := testutils.MustGetConnectedReplSetClient(ctx, t)
	defer client.Disconnect(ctx)

	// GetDatabaseCurrentOpStats requires MongoDB 3.6+
	// Skip this test if the version does not match.
	buildInfo, err := shared.GetBuildInfo(client)
	assert.NoErrorf(t, err, "failed to check MongoDB version")
	if buildInfo.VersionArray[0] < 3 || (buildInfo.VersionArray[0] == 3 && buildInfo.VersionArray[1] < 6) {
		t.Skip("MongoDB is not 3.6+, skipping test that requires $currentOp")
	}

	// run
	threshold := int64(1) // milliseconds.
	stats := GetDatabaseCurrentOpStats(client, threshold)

	// test
	assert.NotNil(t, stats)
}
