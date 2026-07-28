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
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/internal/tu"
)

// Use this for testing because labels like cluster ID are not constant in docker containers
// so we cannot use the real topology labels in tests.
type labelsGetterMock struct{}

func (l labelsGetterMock) baseLabels() map[string]string {
	return map[string]string{}
}

func (l labelsGetterMock) loadLabels(context.Context) error {
	return nil
}

// metricLabels flattens the label pairs of a gathered metric into a map.
func metricLabels(m *dto.Metric) map[string]string {
	labels := make(map[string]string, len(m.GetLabel()))
	for _, label := range m.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	return labels
}

const (
	// minTestShards is the number of shards a cluster needs before a test can
	// observe metrics coming from more than one of them.
	minTestShards = 2
	// shardedTestDocs is large enough that every shard of the test cluster ends up
	// owning documents of the collection.
	shardedTestDocs = 100
)

// shardTestCollection shards dbName.collName over every shard of the test cluster
// reached through mongos, so that $collStats and $indexStats report one document
// per shard. It skips the test only when the cluster itself cannot exercise
// sharding, meaning it has fewer than two shards.
func shardTestCollection(ctx context.Context, t *testing.T, client *mongo.Client, dbName, collName string) {
	t.Helper()

	admin := client.Database("admin")

	var shardList struct {
		Shards []bson.M `bson:"shards"`
	}
	require.NoError(t, admin.RunCommand(ctx, bson.D{{Key: "listShards", Value: 1}}).Decode(&shardList))

	if len(shardList.Shards) < minTestShards {
		t.Skipf("the test cluster has %d shards, at least %d are needed", len(shardList.Shards), minTestShards)
	}

	require.NoError(t, admin.RunCommand(ctx, bson.D{{Key: "enableSharding", Value: dbName}}).Err())

	// A hashed shard key on an empty collection presplits the initial chunks and
	// spreads them over all shards, so every shard owns chunks of the collection.
	shardCmd := bson.D{
		{Key: "shardCollection", Value: dbName + "." + collName},
		{Key: "key", Value: bson.D{{Key: "_id", Value: "hashed"}}},
	}
	require.NoError(t, admin.RunCommand(ctx, shardCmd).Err())
}

//nolint:funlen
func TestConnect(t *testing.T) {
	hostname := "127.0.0.1"
	ctx := context.Background()

	ports := map[string]string{
		"standalone":          tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017"),
		"shard-1 primary":     tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"),
		"shard-1 secondary-1": tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "17002"),
		"shard-1 secondary-2": tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY2_PORT", "17003"),
		"shard-2 primary":     tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"),
		"shard-2 secondary-1": tu.GetenvDefault("TEST_MONGODB_S2_SECONDARY1_PORT", "17005"),
		"shard-2 secondary-2": tu.GetenvDefault("TEST_MONGODB_S2_SECONDARY2_PORT", "17006"),
		"config server 1":     tu.GetenvDefault("TEST_MONGODB_CONFIGSVR1_PORT", "17007"),
		"mongos":              tu.GetenvDefault("TEST_MONGODB_MONGOS_PORT", "17000"),
	}

	t.Run("Connect without SSL", func(t *testing.T) {
		for name, port := range ports {
			exporterOpts := &Opts{
				URI:           fmt.Sprintf("mongodb://%s/admin", net.JoinHostPort(hostname, port)),
				DirectConnect: true,
			}
			client, err := connect(ctx, exporterOpts)
			assert.NoError(t, err, name)
			err = client.Disconnect(ctx)
			assert.NoError(t, err, name)
		}
	})

	//nolint:dupl
	t.Run("Test per-request connection", func(t *testing.T) {
		log := promslog.New(&promslog.Config{})

		exporterOpts := &Opts{
			Logger:         log,
			URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
			GlobalConnPool: false,
			DirectConnect:  true,
		}

		e := New(exporterOpts)

		ts := httptest.NewServer(e.Handler())
		defer ts.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := http.Get(ts.URL) //nolint:noctx
				assert.Nil(t, e.client)
				assert.NoError(t, err)
				g, err := io.ReadAll(res.Body)
				_ = res.Body.Close()
				assert.NoError(t, err)
				assert.NotEmpty(t, g)
			}()
		}

		wg.Wait()
	})

	//nolint:dupl
	t.Run("Test global connection", func(t *testing.T) {
		log := promslog.New(&promslog.Config{})

		exporterOpts := &Opts{
			Logger:         log,
			URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
			GlobalConnPool: true,
			DirectConnect:  true,
		}

		e := New(exporterOpts)

		ts := httptest.NewServer(e.Handler())
		defer ts.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := http.Get(ts.URL) //nolint:noctx
				assert.NotNil(t, e.client)
				assert.NoError(t, err)
				g, err := io.ReadAll(res.Body)
				_ = res.Body.Close()
				assert.NoError(t, err)
				assert.NotEmpty(t, g)
			}()
		}

		wg.Wait()
	})
}

// How this test works?
// When connected to a MongoS instance, the makeRegistry method should skip
// adding replSetGetStatusCollector. To test that, we try to unregister a
// replSetGetStatusCollector and it should return false since it wasn't registered.
// Note: Two Collectors are considered equal if their Describe method yields the
// same set of descriptors.
// unregister will try to Describe to get the descriptors set, and we are using
// DescribeByCollect so, in the logs, you will see an error:
// msg="cannot get replSetGetStatus: replSetGetStatus is not supported through mongos"
// This is correct. Collect is being executed to Describe and Unregister.
func TestMongoS(t *testing.T) {
	hostname := "127.0.0.1"
	ctx := context.Background()

	tests := []struct {
		port string
		want bool
	}{
		{
			port: tu.GetenvDefault("TEST_MONGODB_MONGOS_PORT", "17000"),
			want: false,
		},
		{
			port: tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"),
			want: true,
		},
	}

	for _, test := range tests {
		exporterOpts := &Opts{
			Logger:                 promslog.New(&promslog.Config{}),
			URI:                    fmt.Sprintf("mongodb://%s/admin", net.JoinHostPort(hostname, test.port)),
			DirectConnect:          true,
			GlobalConnPool:         false,
			EnableReplicasetStatus: true,
		}

		client, err := connect(ctx, exporterOpts)
		assert.NoError(t, err)

		e := New(exporterOpts)
		rsgsc := newReplicationSetStatusCollector(ctx, client, e.opts.Logger, e.opts.CompatibleMode, new(labelsGetterMock))

		r := e.makeRegistry(ctx, client, new(labelsGetterMock), *e.opts)

		res := r.Unregister(rsgsc)
		assert.Equal(t, test.want, res, fmt.Sprintf("Port: %v", test.port))
		err = client.Disconnect(ctx)
		assert.NoError(t, err)
	}
}

func generateKerberosConfigFile(t *testing.T) *os.File {
	t.Helper()
	kerberosHost, err := tu.IPForContainer("kerberos")
	require.NoError(t, err)

	config := fmt.Sprintf(`
[libdefaults]
    default_realm = PERCONATEST.COM
    forwardable = true
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ignore_acceptor_hostname = true
    rdns = false
[realms]
    PERCONATEST.COM = {
        kdc_ports = 88
        kdc = %s
    }
[domain_realm]
    .perconatest.com = PERCONATEST.COM
    perconatest.com = PERCONATEST.COM
    %s = PERCONATEST.COM
`, kerberosHost, kerberosHost)
	configFile, err := os.Create(t.TempDir() + "/krb5.conf")
	require.NoError(t, err)

	_, err = configFile.WriteString(config)
	require.NoError(t, err)

	return configFile
}

func TestGSSAPIAuth(t *testing.T) {
	logger := promslog.New(&promslog.Config{})

	mongoHost, err := tu.IPForContainer("psmdb-kerberos")
	require.NoError(t, err)

	configFile := generateKerberosConfigFile(t)
	require.NoError(t, err)
	defer func() {
		_ = configFile.Close()
		t.Setenv("KRB5_CONFIG", "")
	}()

	t.Setenv("KRB5_CONFIG", configFile.Name())
	ctx := context.Background()

	username := "pmm-test%40PERCONATEST.COM"
	password := "password1"
	uri := fmt.Sprintf("mongodb://%s:%s@%s/?authSource=$external&authMechanism=GSSAPI",
		username,
		password,
		net.JoinHostPort(mongoHost, "27017"),
	)
	exporterOpts := &Opts{
		URI:            uri,
		Logger:         logger,
		CollectAll:     true,
		GlobalConnPool: false,
		DirectConnect:  true,
	}

	client, err := connect(ctx, exporterOpts)
	assert.NoError(t, err)

	e := New(exporterOpts)
	nodeType, _ := getNodeType(ctx, client)
	gc := newGeneralCollector(ctx, client, nodeType, e.opts.Logger)
	r := e.makeRegistry(ctx, client, new(labelsGetterMock), *e.opts)

	expected := strings.NewReader(`
		# HELP mongodb_up Whether MongoDB is up.
		# TYPE mongodb_up gauge
		mongodb_up {cluster_role="mongod"} 1` + "\n")

	filter := []string{
		"mongodb_up",
	}
	err = testutil.CollectAndCompare(gc, expected, filter...)
	require.NoError(t, err, "mongodb_up metric should be 1")

	res := r.Unregister(gc)
	assert.True(t, res)
}

func TestMongoUpMetric(t *testing.T) {
	ctx := context.Background()

	type testcase struct {
		URI         string
		clusterRole string
		Want        int
	}

	testCases := []testcase{
		{URI: "mongodb://127.0.0.1:12345/admin", Want: 0},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017")), Want: 1, clusterRole: "mongod"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "27017")), Want: 1, clusterRole: "mongod"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "27017")), Want: 1, clusterRole: "mongod"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_S1_ARBITER_PORT", "27017")), Want: 1, clusterRole: "arbiter"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_MONGOS_PORT", "27017")), Want: 1, clusterRole: "mongos"},
	}

	for _, tc := range testCases {
		t.Run(tc.clusterRole+"/"+tc.URI, func(t *testing.T) {
			exporterOpts := &Opts{
				Logger:           promslog.New(&promslog.Config{}),
				URI:              tc.URI,
				ConnectTimeoutMS: 200,
				DirectConnect:    true,
				GlobalConnPool:   false,
				CollectAll:       true,
			}

			client, err := connect(ctx, exporterOpts)
			if tc.Want == 1 {
				assert.NoError(t, err, "Must be able to connect to %s", tc.URI)
			} else {
				assert.Error(t, err, "Must be unable to connect to %s", tc.URI)
			}

			e := New(exporterOpts)
			nodeType, _ := getNodeType(ctx, client)
			gc := newGeneralCollector(ctx, client, nodeType, e.opts.Logger)
			r := e.makeRegistry(ctx, client, new(labelsGetterMock), *e.opts)

			expected := strings.NewReader(fmt.Sprintf(`
		# HELP mongodb_up Whether MongoDB is up.
		# TYPE mongodb_up gauge
		mongodb_up {cluster_role="%s"} %s`, tc.clusterRole, strconv.Itoa(tc.Want)) + "\n")

			filter := []string{
				"mongodb_up",
			}
			err = testutil.CollectAndCompare(gc, expected, filter...)
			assert.NoError(t, err, "mongodb_up metric should be %d", tc.Want)

			res := r.Unregister(gc)
			assert.Equal(t, true, res)
		})
	}
}
