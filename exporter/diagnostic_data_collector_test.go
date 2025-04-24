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
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/percona/exporter_shared/helpers"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestDiagnosticDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	logger := promslog.New(&promslog.Config{})
	ti := labelsGetterMock{}

	dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, logger.With("component", "test"))
	require.NoError(t, err)

	c := newDiagnosticDataCollector(ctx, client, logger, false, ti, dbBuildInfo)

	prefix := "local.oplog.rs.stats.storageStats.wiredTiger"
	if dbBuildInfo.VersionArray[0] < 7 {
		prefix = "local.oplog.rs.stats.wiredTiger"
	}

	// The last \n at the end of this string is important
	expectedString := fmt.Sprintf(`
	# HELP mongodb_oplog_stats_wt_btree_fixed_record_size %s.btree.fixed-record size
	# TYPE mongodb_oplog_stats_wt_btree_fixed_record_size untyped
	mongodb_oplog_stats_wt_btree_fixed_record_size 0
	# HELP mongodb_oplog_stats_wt_transaction_update_conflicts %s.transaction.update conflicts
	# TYPE mongodb_oplog_stats_wt_transaction_update_conflicts untyped
	mongodb_oplog_stats_wt_transaction_update_conflicts 0`, prefix, prefix)
	expected := strings.NewReader(expectedString + "\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_oplog_stats_wt_btree_fixed_record_size",
		"mongodb_oplog_stats_wt_transaction_update_conflicts",
	}

	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}

func getMongoDBVersionInfo(t *testing.T, containerName string) (string, string) {
	t.Helper()
	imageBaseName, version, err := tu.GetImageNameForContainer(containerName)
	require.NoError(t, err)

	var vendor string
	if strings.HasPrefix(imageBaseName, "percona/") {
		vendor = "Percona"
	} else {
		vendor = "MongoDB"
	}
	return version, vendor
}

func TestCollectorWithCompatibleMode(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		containerName string

		// we use a metrics filter for two reasons:
		// 1. The result is huge
		// 2. We need to check against know values. Don't use metrics that return counters like uptime
		//    or counters like the number of transactions because they won't return a known value to compare
		metricsFilter   []string
		expectedMetrics func() io.Reader
	}{
		{
			name:          "basic metrics",
			containerName: "mongo-1-1",
			metricsFilter: []string{
				"mongodb_mongod_storage_engine",
				"mongodb_version_info",
			},
			expectedMetrics: func() io.Reader {
				version, vendor := getMongoDBVersionInfo(t, "mongo-1-1")

				// The last \n at the end of this string is important
				return strings.NewReader(fmt.Sprintf(`
	# HELP mongodb_mongod_storage_engine The storage engine used by the MongoDB instance
	# TYPE mongodb_mongod_storage_engine gauge
	mongodb_mongod_storage_engine{engine="wiredTiger"} 1
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="Community",mongodb="%s",vendor="%s"} 1`, version, vendor) + "\n")
			},
		},
		{
			name:          "replica set metrics from data-carrying node",
			containerName: "mongo-1-1",
			metricsFilter: []string{
				"mongodb_mongod_storage_engine",
				"mongodb_version_info",
				"mongodb_mongod_replset_number_of_members",
			},
			expectedMetrics: func() io.Reader {
				version, vendor := getMongoDBVersionInfo(t, "mongo-1-1")

				// The last \n at the end of this string is important
				return strings.NewReader(fmt.Sprintf(`
    # HELP mongodb_mongod_replset_number_of_members The number of replica set members.
    # TYPE mongodb_mongod_replset_number_of_members gauge
    mongodb_mongod_replset_number_of_members{set="rs1"} 4
	# HELP mongodb_mongod_storage_engine The storage engine used by the MongoDB instance
	# TYPE mongodb_mongod_storage_engine gauge
	mongodb_mongod_storage_engine{engine="wiredTiger"} 1
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="Community",mongodb="%s",vendor="%s"} 1`, version, vendor) + "\n")
			},
		},
		{
			name:          "replica set metrics on arbiter node",
			containerName: "mongo-1-arbiter",
			metricsFilter: []string{
				"mongodb_mongod_storage_engine",
				"mongodb_version_info",
				"mongodb_mongod_replset_my_state",
				"mongodb_mongod_replset_number_of_members",
			},
			expectedMetrics: func() io.Reader {
				version, vendor := getMongoDBVersionInfo(t, "mongo-1-1")

				// The last \n at the end of this string is important
				return strings.NewReader(fmt.Sprintf(`
    # HELP mongodb_mongod_replset_number_of_members The number of replica set members.
    # TYPE mongodb_mongod_replset_number_of_members gauge
    mongodb_mongod_replset_number_of_members{set="rs1"} 4
    # HELP mongodb_mongod_replset_my_state An integer between 0 and 10 that represents the replica state of the current member
    # TYPE mongodb_mongod_replset_my_state gauge
    mongodb_mongod_replset_my_state{set="rs1"} 7
	# HELP mongodb_mongod_storage_engine The storage engine used by the MongoDB instance
	# TYPE mongodb_mongod_storage_engine gauge
	mongodb_mongod_storage_engine{engine="wiredTiger"} 1
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="Community",mongodb="%s",vendor="%s"} 1`, version, vendor) + "\n")
			},
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			port, err := tu.PortForContainer(tt.containerName)
			require.NoError(t, err)
			client := tu.TestClient(ctx, port, t)
			logger := promslog.New(&promslog.Config{})
			ti := labelsGetterMock{}

			dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, logger.With("component", "test"))
			require.NoError(t, err)

			c := newDiagnosticDataCollector(ctx, client, logger, true, ti, dbBuildInfo)

			err = testutil.CollectAndCompare(c, tt.expectedMetrics(), tt.metricsFilter...)
			assert.NoError(t, err)
		})
	}
}

func TestAllDiagnosticDataCollectorMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	logLevel := promslog.NewLevel()
	err := logLevel.Set("debug")
	require.NoError(t, err)
	logger := promslog.New(&promslog.Config{
		Level: logLevel,
	})
	ti := newTopologyInfo(ctx, client, logger)

	dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, logger.With("component", "test"))
	require.NoError(t, err)

	c := newDiagnosticDataCollector(ctx, client, logger, true, ti, dbBuildInfo)

	reg := prometheus.NewRegistry()
	err = reg.Register(c)
	require.NoError(t, err)
	metrics := helpers.CollectMetrics(c)
	actualMetrics := helpers.ReadMetrics(metrics)
	filters := []string{
		"mongodb_mongod_metrics_cursor_open",
		"mongodb_mongod_metrics_get_last_error_wtimeouts_total",
		"mongodb_mongod_wiredtiger_cache_bytes",
		"mongodb_mongod_wiredtiger_transactions_total",
		"mongodb_mongod_wiredtiger_cache_bytes_total",
		"mongodb_op_counters_total",
		"mongodb_ss_mem_resident",
		"mongodb_ss_mem_virtual",
		"mongodb_ss_metrics_cursor_open",
		"mongodb_ss_metrics_getLastError_wtime_totalMillis",
		"mongodb_ss_opcounters",
		"mongodb_ss_opcountersRepl",
		"mongodb_ss_wt_cache_maximum_bytes_configured",
		"mongodb_ss_wt_cache_modified_pages_evicted",
	}
	actualMetrics = filterMetrics(actualMetrics, filters)
	actualLines := helpers.Format(helpers.WriteMetrics(actualMetrics))
	metricNames := getMetricNames(actualLines)

	sort.Strings(filters)
	for _, want := range filters {
		assert.True(t, metricNames[want], fmt.Sprintf("missing %q metric", want))
	}
}

// errorCountHandler is a custom handler that keeps tracks of the number of errors and warnings that were logged.
// it discards all errors of other levels.
type errorCountHandler struct {
	opts       slog.HandlerOptions
	logRecords []slog.Record
}

func newErrorCountHandler(opts *slog.HandlerOptions) *errorCountHandler {
	h := &errorCountHandler{
		logRecords: make([]slog.Record, 0),
	}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelWarn
	}
	return h
}

func (h *errorCountHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Level == slog.LevelError || r.Level == slog.LevelWarn {
		h.logRecords = append(h.logRecords, r)
	}
	return nil
}

func (h *errorCountHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

func (h *errorCountHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *errorCountHandler) WithGroup(_ string) slog.Handler {
	return h
}

//nolint:funlen
func TestDiagnosticDataErrors(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		containerName   string
		expectedMessage string
	}

	cases := []testCase{
		{
			name:            "authenticated arbiter has warning about missing metrics",
			containerName:   "mongo-2-arbiter",
			expectedMessage: "some metrics might be unavailable on arbiter nodes",
		},
		{
			name:            "authenticated data node has no error in logs",
			containerName:   "mongo-1-1",
			expectedMessage: "",
		},
		{
			name:            "unauthenticated arbiter has warning about missing metrics",
			containerName:   "mongo-1-arbiter",
			expectedMessage: "some metrics might be unavailable on arbiter nodes",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			port, err := tu.PortForContainer(tc.containerName)
			require.NoError(t, err)
			client := tu.TestClient(ctx, port, t)

			errCountLogHandler := newErrorCountHandler(nil)
			logger := slog.New(errCountLogHandler)
			ti := newTopologyInfo(ctx, client, logger)

			dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, logger.With("component", "test"))
			require.NoError(t, err)

			c := newDiagnosticDataCollector(ctx, client, logger, true, ti, dbBuildInfo)

			reg := prometheus.NewRegistry()
			err = reg.Register(c)
			require.NoError(t, err)
			_ = helpers.CollectMetrics(c)

			if tc.expectedMessage == "" {
				assert.Empty(t, errCountLogHandler.logRecords)
			} else {
				require.NotEmpty(t, errCountLogHandler.logRecords)
				messages := make([]string, 0, len(errCountLogHandler.logRecords))
				for _, record := range errCountLogHandler.logRecords {
					messages = append(messages, record.Message)
				}
				assert.Contains(t, messages, tc.expectedMessage)
			}
		})
	}
}

func TestContextTimeout(t *testing.T) {
	ctx := context.Background()

	client := tu.DefaultTestClient(ctx, t)

	logger := promslog.New(&promslog.Config{})
	ti := newTopologyInfo(ctx, client, logger)

	dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, logger.With("component", "test"))
	require.NoError(t, err)

	dbCount := 100

	err = addTestData(ctx, client, dbCount)
	assert.NoError(t, err)

	defer cleanTestData(ctx, client, dbCount) //nolint:errcheck

	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()

	c := newDiagnosticDataCollector(cctx, client, logger, true, ti, dbBuildInfo)
	// it should not panic
	helpers.CollectMetrics(c)
}

func addTestData(ctx context.Context, client *mongo.Client, count int) error {
	session, err := client.StartSession()
	if err != nil {
		return errors.Wrap(err, "cannot create session to add test data")
	}

	if err := session.StartTransaction(); err != nil {
		return errors.Wrap(err, "cannot start session to add test data")
	}

	if err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		for i := 0; i < count; i++ {
			dbName := fmt.Sprintf("testdb_%06d", i)
			doc := bson.D{{Key: "field1", Value: "value 1"}}
			_, err := client.Database(dbName).Collection("test_col").InsertOne(ctx, doc)
			if err != nil {
				return errors.Wrap(err, "cannot add test data")
			}
		}

		if err = session.CommitTransaction(ctx); err != nil {
			return errors.Wrap(err, "cannot commit add test data transaction")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "cannot add data inside a session")
	}

	session.EndSession(ctx)

	return nil
}

func cleanTestData(ctx context.Context, client *mongo.Client, count int) error {
	session, err := client.StartSession()
	if err != nil {
		return errors.Wrap(err, "cannot create session to add test data")
	}

	if err := session.StartTransaction(); err != nil {
		return errors.Wrap(err, "cannot start session to add test data")
	}

	if err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		for i := 0; i < count; i++ {
			dbName := fmt.Sprintf("testdb_%06d", i)
			client.Database(dbName).Drop(ctx) //nolint:errcheck
		}

		if err = session.CommitTransaction(sc); err != nil {
			return errors.Wrap(err, "cannot commit add test data transaction")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "cannot add data inside a session")
	}

	session.EndSession(ctx)

	return nil
}

func TestDisconnectedDiagnosticDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	err := client.Disconnect(ctx)
	assert.NoError(t, err)

	logger := promslog.NewNopLogger()

	ti := labelsGetterMock{}

	dbBuildInfo, err := retrieveMongoDBBuildInfo(ctx, client, logger.With("component", "test"))
	require.Error(t, err)

	c := newDiagnosticDataCollector(ctx, client, logger, true, ti, dbBuildInfo)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="",mongodb="",vendor=""} 1` + "\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_version_info",
	}

	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
