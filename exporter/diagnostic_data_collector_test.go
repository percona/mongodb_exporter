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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/percona/exporter_shared/helpers"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
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
	logger := logrus.New()
	ti := labelsGetterMock{}

	c := newDiagnosticDataCollector(ctx, client, logger, false, ti)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
	# HELP mongodb_oplog_stats_ok local.oplog.rs.stats.
	# TYPE mongodb_oplog_stats_ok untyped
	mongodb_oplog_stats_ok 1
	# HELP mongodb_oplog_stats_wt_btree_fixed_record_size local.oplog.rs.stats.wiredTiger.btree.
	# TYPE mongodb_oplog_stats_wt_btree_fixed_record_size untyped
	mongodb_oplog_stats_wt_btree_fixed_record_size 0
	# HELP mongodb_oplog_stats_wt_transaction_update_conflicts local.oplog.rs.stats.wiredTiger.transaction.
	# TYPE mongodb_oplog_stats_wt_transaction_update_conflicts untyped
	mongodb_oplog_stats_wt_transaction_update_conflicts 0` + "\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_oplog_stats_ok",
		"mongodb_oplog_stats_wt_btree_fixed_record_size",
		"mongodb_oplog_stats_wt_transaction_update_conflicts",
	}

	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}

func TestDiagnosticDataCollectorWithCompatibleMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	logger := logrus.New()
	ti := labelsGetterMock{}

	imageBaseName, version, err := tu.GetImageNameForDefault()
	require.NoError(t, err)

	var vendor string
	if strings.HasPrefix(imageBaseName, "percona/") {
		vendor = "Percona"
	} else {
		vendor = "MongoDB"
	}

	c := newDiagnosticDataCollector(ctx, client, logger, true, ti)

	// The last \n at the end of this string is important
	expected := strings.NewReader(fmt.Sprintf(`
	# HELP mongodb_mongod_storage_engine The storage engine used by the MongoDB instance
	# TYPE mongodb_mongod_storage_engine gauge
	mongodb_mongod_storage_engine{engine="wiredTiger"} 1
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="Community",mongodb="%s",vendor="%s"} 1`, version, vendor) + "\n")

	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_mongod_storage_engine",
		"mongodb_version_info",
	}

	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}

func getMongoDBVersion(t *testing.T, client *mongo.Client, ctx context.Context, logger *logrus.Logger) (string, error) {
	var m bson.M
	cmd := bson.D{{Key: "getDiagnosticData", Value: "1"}}
	res := client.Database("admin").RunCommand(ctx, cmd)
	if res.Err() != nil {
		return "", res.Err()
	}

	if err := res.Decode(&m); err != nil {
		logger.Errorf("cannot run getDiagnosticData: %s", err)
	}

	m, ok := m["data"].(bson.M)
	if !ok {
		return "", errors.New("cannot decode getDiagnosticData")
	}

	v := walkTo(m, []string{"serverStatus", "version"})
	serverVersion, ok := v.(string)
	if !ok {
		serverVersion = "server version is unavailable"
	}
	return serverVersion, nil
}

func TestAllDiagnosticDataCollectorMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	ti := newTopologyInfo(ctx, client, logrus.New())

	c := newDiagnosticDataCollector(ctx, client, logrus.New(), true, ti)

	reg := prometheus.NewRegistry()
	err := reg.Register(c)
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

func TestContextTimeout(t *testing.T) {
	ctx := context.Background()

	client := tu.DefaultTestClient(ctx, t)

	ti := newTopologyInfo(ctx, client, logrus.New())

	dbCount := 100

	err := addTestData(ctx, client, dbCount)
	assert.NoError(t, err)

	defer cleanTestData(ctx, client, dbCount) //nolint:errcheck

	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()

	c := newDiagnosticDataCollector(cctx, client, logrus.New(), true, ti)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	err := client.Disconnect(ctx)
	assert.NoError(t, err)

	logger := logrus.New()
	logger.Out = io.Discard // diable logs in tests

	ti := labelsGetterMock{}

	c := newDiagnosticDataCollector(ctx, client, logger, true, ti)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
	# HELP mongodb_mongod_replset_my_state An integer between 0 and 10 that represents the replica state of the current member
	# TYPE mongodb_mongod_replset_my_state gauge
	mongodb_mongod_replset_my_state{set=""} 6
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="",mongodb="",vendor=""} 1` + "\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_mongod_replset_my_state",
		"mongodb_mongod_storage_engine",
		"mongodb_version_info",
	}

	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
