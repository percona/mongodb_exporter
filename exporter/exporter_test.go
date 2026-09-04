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
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/internal/tu"
)

const (
	// minTestShards is the number of shards a cluster needs before a test can
	// observe metrics coming from more than one of them.
	minTestShards = 2
	// shardedTestDocs is large enough that every shard of the test cluster ends up
	// owning documents of the collection.
	shardedTestDocs = 100
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

// gatherShardedMetrics shards dbName.collName over the test cluster, fills it with documents so
// that every shard reports on it, and returns what c exposes for it. The collector is expected
// to be scoped to that one namespace.
func gatherShardedMetrics(ctx context.Context, t *testing.T, client *mongo.Client,
	dbName, collName string, c prometheus.Collector,
) []*dto.MetricFamily {
	t.Helper()

	shardTestCollection(ctx, t, client, dbName, collName)

	docs := make([]any, 0, shardedTestDocs)
	for i := range shardedTestDocs {
		docs = append(docs, bson.M{"f1": i})
	}
	_, err := client.Database(dbName).Collection(collName).InsertMany(ctx, docs)
	require.NoError(t, err)

	// Register runs Describe, which collects everything, and rejects a metric name described
	// with two different label sets. That is how a shard label set for only some of the
	// documents would surface.
	registry := prometheus.NewPedanticRegistry()
	require.NoError(t, registry.Register(c))

	families, err := registry.Gather()
	require.NoError(t, err)

	return families
}

// shardedSeriesLabels returns the labels of every series of dbName.collName exposed by the metric
// families whose name starts with namePrefix, asserting that each of them carries a shard.
func shardedSeriesLabels(t *testing.T, families []*dto.MetricFamily,
	namePrefix, dbName, collName string,
) []map[string]string {
	t.Helper()

	var series []map[string]string
	for _, family := range families {
		if !strings.HasPrefix(family.GetName(), namePrefix) {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := metricLabels(metric)
			if labels["database"] != dbName || labels["collection"] != collName {
				continue
			}

			require.NotEmpty(t, labels["shard"], "series without a shard label: %s %v", family.GetName(), labels)
			series = append(series, labels)
		}
	}

	require.NotEmpty(t, series, "no %s* series for %s.%s", namePrefix, dbName, collName)

	return series
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
				assert.Nil(t, cachedClient(e))
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
				assert.NotNil(t, cachedClient(e))
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

// newPooledExporter builds an exporter that reuses one client. It skips New, whose
// background initial connect would race with tests that drive getClient themselves.
func newPooledExporter(t *testing.T) *Exporter {
	t.Helper()

	log := promslog.New(&promslog.Config{})

	opts := &Opts{
		Logger:         log,
		URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
		GlobalConnPool: true,
		DirectConnect:  true,
	}

	return &Exporter{
		logger:                log,
		opts:                  opts,
		lock:                  &sync.Mutex{},
		totalCollectionsCount: -1,
	}
}

// cachedClient returns the pooled client, or nil if none has been built yet.
func cachedClient(e *Exporter) *mongo.Client {
	if pooled := e.client.Load(); pooled != nil {
		return pooled.Client
	}

	return nil
}

// blackHoleMongo returns the address of a listener that accepts connections and never
// answers, so a driver handshake against it blocks until its own timeout rather than
// failing fast the way a closed port would. The returned channel is closed once the first
// connection has been accepted, which is when a connect is provably in flight, and the
// counter says how many were accepted in all, which tells a hung handshake from a redial loop.
func blackHoleMongo(t *testing.T) (string, <-chan struct{}, *atomic.Int32) {
	t.Helper()

	var listenCfg net.ListenConfig
	listener, err := listenCfg.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	dialed := make(chan struct{})
	var accepts atomic.Int32
	go func() {
		// Every accepted connection is kept until the listener closes. Dropping the reference
		// would let the net package's finalizer close it at the next GC, and the driver would
		// see a reset instead of a hung handshake.
		var conns []net.Conn
		defer func() {
			for _, conn := range conns {
				_ = conn.Close()
			}
		}()

		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conns = append(conns, conn)
			if accepts.Add(1) == 1 {
				close(dialed)
			}
		}
	}()

	return listener.Addr().String(), dialed, &accepts
}

// syncBuffer collects log output written from a flight's goroutine while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// bytes.Buffer.Write documents that its error is always nil.
	n, _ := b.buf.Write(p)

	return n, nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// A client whose health check keeps failing is dropped, so the next scrape builds a new one.
// Nothing else ever replaces the pooled client, so this is what picks up changes the driver
// reads once, such as rotated TLS material. One failure is not enough: a server mid-election
// recovers on its own, and rebuilding would only add churn.
func TestGlobalConnPoolDropsClientAfterRepeatedPingFailures(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	e := newPooledExporter(t)
	addr, _, _ := blackHoleMongo(t)
	e.opts.URI = "mongodb://" + addr + "/admin"
	e.opts.ConnectTimeoutMS = 200

	// A client that cannot reach its server fails every health check with a selection error.
	// That is the transient-looking failure the counter exists for, as opposed to the
	// disconnected sentinel, which is dropped on sight and so would not exercise it.
	clientOpts, _, err := clientOptionsFor(e.opts)
	require.NoError(t, err)
	unreachable, err := mongo.Connect(ctx, clientOpts)
	require.NoError(t, err)
	e.client.Store(&pooledClient{Client: unreachable})

	for range maxConsecutivePingFailures - 1 {
		_, err = e.getClient(ctx)
		require.Error(t, err)
		require.Same(t, unreachable, cachedClient(e), "client was dropped before the failures added up")
	}

	_, err = e.getClient(ctx)
	require.Error(t, err)
	require.Nil(t, cachedClient(e), "failing client stayed cached")
}

// A disconnected client never recovers, so waiting for it to fail the count out would report
// mongodb_up 0 for scrapes that could have been served by a new client.
func TestGlobalConnPoolDropsDisconnectedClientAtOnce(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	e := newPooledExporter(t)

	first, err := e.getClient(ctx)
	require.NoError(t, err)
	require.NoError(t, first.Disconnect(ctx))

	_, err = e.getClient(ctx)
	require.Error(t, err)
	require.Nil(t, cachedClient(e), "disconnected client stayed cached")

	second, err := e.getClient(ctx)
	require.NoError(t, err)
	assert.NotSame(t, first, second)
	assert.NoError(t, second.Ping(ctx, nil))

	require.NoError(t, second.Disconnect(ctx))
}

// The pooled client is built during whichever scrape happens to find the cache empty, but
// it has to outlive that scrape. This holds because the driver connects the topology
// without a context and does not retain the one passed to mongo.Connect.
func TestGlobalConnPoolClientOutlivesCreatingScrape(t *testing.T) {
	t.Parallel()

	e := newPooledExporter(t)

	scrape, cancel := context.WithCancel(t.Context())
	first, err := e.getClient(scrape)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Disconnect(context.Background()) })

	// The scrape that created the client ends.
	cancel()

	second, err := e.getClient(t.Context())
	require.NoError(t, err)
	assert.Same(t, first, second, "pooled client was lost when its creating scrape ended")
	assert.NoError(t, second.Ping(t.Context(), nil))
}

// Giving up on the scrape budget must not cancel the connect: a budget shorter than one
// connect would otherwise leave the pool permanently empty, so every scrape would keep
// paying for an attempt that can never finish.
func TestGlobalConnPoolCacheWarmsAfterScrapeGivesUp(t *testing.T) {
	t.Parallel()

	e := newPooledExporter(t)
	// Keep the flight's budget under the wait below, so a flight still running when the test
	// ends cannot land a client nobody disconnects.
	e.opts.ConnectTimeoutMS = 5000
	t.Cleanup(func() {
		if client := cachedClient(e); client != nil {
			_ = client.Disconnect(context.Background())
		}
	})

	// No budget at all, so the scrape cannot wait for the connect it starts.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := e.getClient(ctx)
	require.Error(t, err)

	require.Eventually(t, func() bool {
		return cachedClient(e) != nil
	}, 15*time.Second, 50*time.Millisecond,
		"the connect was cancelled along with the scrape, leaving the pool empty")
}

// A scrape must not wait out a connect somebody else started -- the one New runs in the
// background, or another scrape's -- even though that connect is deliberately given a
// budget of its own, longer than any single scrape's.
func TestGlobalConnPoolScrapeGivesUpOnConnectInFlight(t *testing.T) {
	t.Parallel()

	e := newPooledExporter(t)
	addr, dialed, accepts := blackHoleMongo(t)
	e.opts.URI = "mongodb://" + addr + "/admin"
	e.opts.ConnectTimeoutMS = 3000

	inFlight := make(chan struct{})
	go func() {
		defer close(inFlight)
		_, _ = e.getClient(context.Background())
	}()

	// Only once the listener has accepted is a connect provably under way. Without this the
	// scrape below could win the race and simply do its own connect.
	select {
	case <-dialed:
	case <-time.After(10 * time.Second):
		t.Fatal("the background connect never dialled")
	}

	budget := 300 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), budget)
	defer cancel()

	start := time.Now()
	_, err := e.getClient(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 4*budget,
		"scrape waited for the in-flight connect instead of giving up on its budget")

	// Well past the flight's budget of connect plus selection timeout.
	select {
	case <-inFlight:
	case <-time.After(10 * time.Second):
		t.Fatal("the background connect never returned")
	}

	// A handshake the listener holds open is dialled once, or twice if the heartbeat times out
	// and redials before selection gives up. A fixture that let the socket go would have handed
	// the driver a reset every half second instead.
	assert.LessOrEqual(t, accepts.Load(), int32(2), "the black hole did not hold the connection open")
}

// A flight nobody waits on any more still has to say why it failed: the scrapes that started
// it logged only their own deadline, and nothing else sees the cause.
func TestGlobalConnPoolLogsConnectFailureAfterScrapesGaveUp(t *testing.T) {
	t.Parallel()

	var logs syncBuffer
	e := newPooledExporter(t)
	e.logger = slog.New(slog.NewTextHandler(&logs, nil))
	addr, _, _ := blackHoleMongo(t)
	e.opts.URI = "mongodb://" + addr + "/admin"
	e.opts.ConnectTimeoutMS = 1000

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := e.getClient(ctx)
	require.ErrorIs(t, err, context.Canceled)

	require.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "MongoDB connect attempt failed")
	}, 5*time.Second, 50*time.Millisecond, "the flight's failure was never logged")
	assert.Contains(t, logs.String(), "server selection error", "the log carries the cause, not just a deadline")
}

// singleflight re-raises a panic from a flight on a goroutine of its own, where nothing can
// recover it, so a connect that panics has to come back as an error from inside the flight.
func TestGlobalConnPoolBuildTurnsPanicIntoError(t *testing.T) {
	t.Parallel()

	// Nil options are the cheapest panic to hand the flight.
	e := &Exporter{logger: promslog.New(&promslog.Config{})}

	res := <-e.clientGroup.DoChan("", e.buildClient)
	require.ErrorIs(t, res.Err, errConnectPanicked)
}

// New warms the pool in the background only when there is a pool to warm. Without one a
// connect at startup serves nobody, since every scrape builds its own client.
func TestNewConnectsAtStartupOnlyForPool(t *testing.T) {
	t.Parallel()

	for name, pool := range map[string]bool{"pool on": true, "pool off": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			addr, dialed, _ := blackHoleMongo(t)
			New(&Opts{
				Logger:           promslog.New(&promslog.Config{}),
				URI:              "mongodb://" + addr + "/admin",
				GlobalConnPool:   pool,
				DirectConnect:    true,
				ConnectTimeoutMS: 1000,
			})

			select {
			case <-dialed:
				assert.True(t, pool, "startup connected although every scrape builds its own client")
			case <-time.After(2 * time.Second):
				assert.False(t, pool, "the pool was not warmed at startup")
			}
		})
	}
}

// A scrape that runs out of time must not cost us the pool: the client is healthy, only that
// request is over. This has to hold however many of them expire, because concurrent scrapes
// of one target run out independently -- counting them would let three that expire together
// evict a client with nothing wrong with it.
func TestGlobalConnPoolKeepsClientOnTransientError(t *testing.T) {
	t.Parallel()

	e := newPooledExporter(t)

	first, err := e.getClient(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Disconnect(context.Background()) })

	for range maxConsecutivePingFailures + 1 {
		expired, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = e.getClient(expired)
		require.Error(t, err)
		require.Same(t, first, cachedClient(e), "healthy client was dropped after a scrape ran out of time")
	}

	assert.NoError(t, first.Ping(t.Context(), nil), "the client the scrapes gave up on is still usable")
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
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001")), Want: 1, clusterRole: "mongod"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "17002")), Want: 1, clusterRole: "mongod"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_S1_ARBITER_PORT", "17011")), Want: 1, clusterRole: "arbiter"},
		{URI: fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.GetenvDefault("TEST_MONGODB_MONGOS_PORT", "17000")), Want: 1, clusterRole: "mongos"},
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

// The connect budget has to be one number, used both for the driver's own timeouts and for
// the deadline a caller wraps around the attempt. When they disagreed, the outer deadline
// silently won: a URI asking for more than --mongodb.connect-timeout-ms had its handshake
// aborted on every scrape, so mongodb_up stayed 0 permanently rather than transiently.
//
//nolint:funlen
func TestClientOptionsForResolvesOneConnectBudget(t *testing.T) {
	t.Parallel()

	const uri = "mongodb://127.0.0.1:27017/admin"

	tests := map[string]struct {
		uri              string
		connectTimeoutMS int
		wantConnect      time.Duration
		wantSelection    time.Duration
		wantBudget       time.Duration
	}{
		// connectTimeoutMS says nothing about how long selection may take, so selection keeps
		// the driver's default. Deriving it from the connect timeout would shrink the window a
		// scrape needs to ride out an election.
		"connect timeout in the uri keeps the default selection window": {
			uri:              uri + "?connectTimeoutMS=8000",
			connectTimeoutMS: 5000,
			wantConnect:      8 * time.Second,
			wantSelection:    defaultConnectTimeout,
			wantBudget:       8*time.Second + defaultConnectTimeout,
		},
		// A zero connect timeout is the driver's "no dial timeout", not an unset one: the flag
		// must not replace it, nor selection be narrowed as if the URI had said nothing.
		"zero connect timeout in the uri is passed through": {
			uri:              uri + "?connectTimeoutMS=0",
			connectTimeoutMS: 5000,
			wantConnect:      0,
			wantSelection:    defaultConnectTimeout,
			wantBudget:       defaultConnectTimeout,
		},
		"flag applies when the uri is silent": {
			uri:              uri,
			connectTimeoutMS: 5000,
			wantConnect:      5 * time.Second,
			wantSelection:    5 * time.Second,
			wantBudget:       10 * time.Second,
		},
		"explicit selection timeout in the uri is left alone": {
			uri:              uri + "?connectTimeoutMS=8000&serverSelectionTimeoutMS=2000",
			connectTimeoutMS: 5000,
			wantConnect:      8 * time.Second,
			wantSelection:    2 * time.Second,
			wantBudget:       10 * time.Second,
		},
		// The reverse ordering is the one that used to break: taking the budget from the
		// connect side alone expired the caller's deadline at 2s while the operator's 8s
		// selection window was still running, so the scrape reported its own deadline
		// rather than the setting.
		"selection timeout longer than connect widens the budget": {
			uri:              uri + "?connectTimeoutMS=2000&serverSelectionTimeoutMS=8000",
			connectTimeoutMS: 5000,
			wantConnect:      2 * time.Second,
			wantSelection:    8 * time.Second,
			wantBudget:       10 * time.Second,
		},
		"selection timeout alone widens the budget past the flag": {
			uri:              uri + "?serverSelectionTimeoutMS=8000",
			connectTimeoutMS: 5000,
			wantConnect:      5 * time.Second,
			wantSelection:    8 * time.Second,
			wantBudget:       13 * time.Second,
		},
		// The driver would select forever, and the connect with it, leaving the pool empty for
		// good. The driver is told the fallback too, so the caller's deadline agrees with it.
		"zero selection timeout in the uri falls back rather than asking for no timeout": {
			uri:              uri + "?connectTimeoutMS=5000&serverSelectionTimeoutMS=0",
			connectTimeoutMS: 5000,
			wantConnect:      5 * time.Second,
			wantSelection:    defaultConnectTimeout,
			wantBudget:       5*time.Second + defaultConnectTimeout,
		},
		"neither set falls back rather than asking for no timeout": {
			uri:              uri,
			connectTimeoutMS: 0,
			wantConnect:      defaultConnectTimeout,
			wantSelection:    defaultConnectTimeout,
			wantBudget:       2 * defaultConnectTimeout,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			clientOpts, budget, err := clientOptionsFor(&Opts{URI: test.uri, ConnectTimeoutMS: test.connectTimeoutMS})
			require.NoError(t, err)

			require.NotNil(t, clientOpts.ConnectTimeout)
			assert.Equal(t, test.wantConnect, *clientOpts.ConnectTimeout)
			require.NotNil(t, clientOpts.ServerSelectionTimeout,
				"unset would leave the driver to decide, and the budget could not follow it")
			assert.Equal(t, test.wantSelection, *clientOpts.ServerSelectionTimeout)
			assert.Equal(t, test.wantBudget, budget, "budget a caller would bound the attempt with")
		})
	}
}

// The driver never prunes idle connections by default and reuses them without checking they
// are still open, so a socket a middlebox dropped between scrapes would fail the next one.
func TestClientOptionsForPrunesIdleConnections(t *testing.T) {
	t.Parallel()

	const uri = "mongodb://127.0.0.1:27017/admin"

	clientOpts, _, err := clientOptionsFor(&Opts{URI: uri})
	require.NoError(t, err)
	require.NotNil(t, clientOpts.MaxConnIdleTime, "idle connections would never be pruned")
	assert.Equal(t, defaultMaxConnIdleTime, *clientOpts.MaxConnIdleTime)

	clientOpts, _, err = clientOptionsFor(&Opts{URI: uri + "?maxIdleTimeMS=1000"})
	require.NoError(t, err)
	require.NotNil(t, clientOpts.MaxConnIdleTime)
	assert.Equal(t, time.Second, *clientOpts.MaxConnIdleTime, "the uri's own setting was overridden")
}

// A flight must never displace a client already in the cache. singleflight retires its key
// when a flight returns, so a scrape that read an empty cache and arrives after an earlier
// flight finished starts a second flight rather than joining the first. Connecting again
// would strand the cached client: nothing in the pooled path ever disconnects one, so its
// topology, monitor goroutines and heartbeat connections would outlive it -- background
// load on the very MongoDB this pool exists to take load off.
func TestGlobalConnPoolBuildDoesNotOrphanCachedClient(t *testing.T) {
	t.Parallel()

	e := newPooledExporter(t)

	first, err := e.getClient(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Disconnect(context.Background()) })

	// Exactly what that second flight runs.
	got, err := e.buildClient()
	require.NoError(t, err)

	assert.Same(t, first, got, "the flight connected again instead of taking the cached client")
	assert.Same(t, first, cachedClient(e), "the cached client was displaced, and nothing disconnects it")
}
