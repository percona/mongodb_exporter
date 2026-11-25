// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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
	"net"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:gochecknoglobals
var (
	testDBs   = []string{"testdb01", "testdb02"}
	testColls = []string{"col01", "col02", "colxx", "colyy"}
	testViews = []string{"view01", "view02"}
)

func setupDB(ctx context.Context, t *testing.T, client *mongo.Client) {
	t.Helper()

	for _, dbname := range testDBs {
		for _, coll := range testColls {
			for j := 0; j < 10; j++ {
				_, err := client.Database(dbname).Collection(coll).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
				assert.NoError(t, err)
			}
		}
	}
	for _, view := range testViews {
		err := client.Database(testDBs[0]).CreateView(ctx, view, testColls[0], mongo.Pipeline{})
		assert.NoError(t, err)
	}
}

func cleanupDB(ctx context.Context, client *mongo.Client) {
	for _, dbname := range testDBs {
		client.Database(dbname).Drop(ctx) //nolint:errcheck
	}
}

//nolint:paralleltest
func TestListDatabases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	setupDB(ctx, t, client)
	defer cleanupDB(ctx, client)

	t.Run("Empty filter in list", func(t *testing.T) {
		want := []string{"testdb01", "testdb02"}
		allDBs, err := databases(ctx, client, promslog.NewNopLogger(), nil, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, want, allDBs)
	})

	t.Run("One collection in list", func(t *testing.T) {
		want := []string{"testdb01"}
		allDBs, err := databases(ctx, client, promslog.NewNopLogger(), []string{"testdb01.col1"}, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, want, allDBs)
	})

	t.Run("Multiple namespaces in list", func(t *testing.T) {
		want := []string{"testdb01", "testdb02"}
		allDBs, err := databases(ctx, client, promslog.NewNopLogger(), []string{"testdb01", "testdb02.col2", "testdb02.col1"}, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, want, allDBs)
	})
}

//nolint:paralleltest
func TestListCollections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	setupDB(ctx, t, client)
	defer cleanupDB(ctx, client)

	t.Run("Basic test", func(t *testing.T) {
		want := []string{"admin", "config", "local", "testdb01", "testdb02"}
		allDBs, err := databases(ctx, client, promslog.NewNopLogger(), nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, want, allDBs)
	})

	t.Run("Filter in databases", func(t *testing.T) {
		want := []string{"col01", "col02", "colxx"}
		inNameSpaces := []string{testDBs[0] + ".col0", testDBs[0] + ".colx"}
		colls, err := listCollections(ctx, client, testDBs[0], inNameSpaces, true)
		sort.Strings(colls)

		assert.NoError(t, err)
		assert.Equal(t, want, colls)
	})

	t.Run("With namespaces list", func(t *testing.T) {
		// Advanced filtering test
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy", "system.views"},
			"testdb02": {"col01", "col02"},
		}
		// List all collections in testdb01 (inDBs[0]) but only col01 and col02 from testdb02.
		filterInNameSpaces := []string{testDBs[0], testDBs[1] + ".col01", testDBs[1] + ".col02"}
		namespaces, err := listAllCollections(ctx, client, promslog.NewNopLogger(), filterInNameSpaces, systemDBs, true)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Empty namespaces list", func(t *testing.T) {
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy", "system.views"},
			"testdb02": {"col01", "col02", "colxx", "colyy"},
		}
		namespaces, err := listAllCollections(ctx, client, promslog.NewNopLogger(), nil, systemDBs, true)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Collections with views", func(t *testing.T) {
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy", "system.views", "view01", "view02"},
			"testdb02": {"col01", "col02", "colxx", "colyy"},
		}
		namespaces, err := listAllCollections(ctx, client, promslog.NewNopLogger(), nil, systemDBs, false)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Count basic", func(t *testing.T) {
		count, complete, err := nonSystemCollectionsCount(ctx, client, promslog.NewNopLogger(), nil)
		assert.NoError(t, err)
		assert.True(t, complete)
		assert.Equal(t, 9, count)
	})

	t.Run("Filtered count", func(t *testing.T) {
		count, complete, err := nonSystemCollectionsCount(ctx, client, promslog.NewNopLogger(), []string{testDBs[0] + ".col0", testDBs[0] + ".colx"})
		assert.NoError(t, err)
		assert.True(t, complete)
		assert.Equal(t, 6, count)
	})
}

func proxyTestClient(ctx context.Context, t *testing.T, uri string) *mongo.Client {
	t.Helper()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetDirect(true).
		SetServerSelectionTimeout(5*time.Second))
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Disconnect(ctx) //nolint:errcheck
	})

	return client
}

// Some managed services reject listDatabases arguments outside an allow-list (issue #597, #412).
//
//nolint:paralleltest
func TestListDatabasesStrictProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	setupDB(ctx, t, client)
	defer cleanupDB(ctx, client)

	port, err := tu.PortForContainer("mongo-1-1")
	require.NoError(t, err)

	for name, reject := range map[string]func(bson.Raw) string{
		"authorizedDatabases rejected":        rejectArgs("listDatabases", "authorizedDatabases"),
		"filter rejected":                     rejectArgs("listDatabases", "filter"),
		"connectionStatus rejected (no auth)": rejectAny(rejectArgs("listDatabases", "authorizedDatabases"), rejectArgs("connectionStatus")),
	} {
		t.Run(name, func(t *testing.T) {
			proxy := startStrictProxy(ctx, t, net.JoinHostPort("127.0.0.1", port), reject)
			proxyClient := proxyTestClient(ctx, t, "mongodb://"+proxy.addr)

			allDBs, err := databases(ctx, proxyClient, promslog.NewNopLogger(), nil, systemDBs)
			require.NoError(t, err)
			assert.Equal(t, testDBs, allDBs)

			allDBs, err = databases(ctx, proxyClient, promslog.NewNopLogger(), []string{"testdb02.col01"}, systemDBs)
			require.NoError(t, err)
			assert.Equal(t, []string{"testdb02"}, allDBs)

			assert.Positive(t, proxy.rejected.Load(), "the proxy should have rejected the command")
		})
	}
}

// A network error on listDatabases must be returned, not trigger the fallback for proxies.
//
//nolint:paralleltest
func TestListDatabasesNetworkError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	port, err := tu.PortForContainer("mongo-1-1")
	require.NoError(t, err)

	var fallbackSent atomic.Bool

	proxy := startStrictProxy(ctx, t, net.JoinHostPort("127.0.0.1", port), func(body bson.Raw) string {
		switch {
		case rejectArgs("listDatabases", "authorizedDatabases")(body) != "":
			return dropConnection
		case commandName(body) == "listDatabases", commandName(body) == "connectionStatus":
			fallbackSent.Store(true)
		}

		return ""
	})
	client := proxyTestClient(ctx, t, "mongodb://"+proxy.addr)
	require.NoError(t, client.Ping(ctx, nil))

	_, err = databases(ctx, client, promslog.NewNopLogger(), nil, nil)
	require.Error(t, err)
	assert.True(t, mongo.IsNetworkError(err), "want a network error, got %v", err)
	assert.False(t, fallbackSent.Load(), "a network error must not trigger the fallback")
}

// When the fallback fails too, both errors must be reported.
//
//nolint:paralleltest
func TestListDatabasesFallbackError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	port, err := tu.PortForContainer("mongo-1-1")
	require.NoError(t, err)

	proxy := startStrictProxy(ctx, t, net.JoinHostPort("127.0.0.1", port), func(body bson.Raw) string {
		if msg := rejectArgs("listDatabases", "authorizedDatabases")(body); msg != "" {
			return msg
		}
		if commandName(body) == "listDatabases" {
			return "bare listDatabases rejected"
		}

		return ""
	})
	client := proxyTestClient(ctx, t, "mongodb://"+proxy.addr)

	_, err = databases(ctx, client, promslog.NewNopLogger(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unallowed argument in listDatabases command: authorizedDatabases")
	assert.Contains(t, err.Error(), "bare listDatabases rejected")
}

// A user with the listDatabases privilege but privileges on only some databases must only get
// those databases, also when a proxy rejects authorizedDatabases.
//
//nolint:paralleltest,funlen
func TestListDatabasesUserPrivileges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	const (
		allowedDB = "privtest_allowed"
		deniedDB  = "privtest_denied"
		role      = "privtestRole"
		user      = "privtest"
		password  = "privtest"
	)

	addr := net.JoinHostPort("127.0.0.1", tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"))
	admin := proxyTestClient(ctx, t, "mongodb://admin:admin@"+addr)

	for _, db := range []string{allowedDB, deniedDB} {
		for _, coll := range []string{"c1", "c2"} {
			_, err := admin.Database(db).Collection(coll).InsertOne(ctx, bson.M{"f": 1})
			require.NoError(t, err)
		}
	}

	adminDB := admin.Database("admin")
	adminDB.RunCommand(ctx, bson.D{{Key: "dropUser", Value: user}})
	adminDB.RunCommand(ctx, bson.D{{Key: "dropRole", Value: role}})
	require.NoError(t, adminDB.RunCommand(ctx, bson.D{
		{Key: "createRole", Value: role},
		{Key: "privileges", Value: bson.A{
			bson.M{"resource": bson.M{"cluster": true}, "actions": bson.A{"listDatabases", "serverStatus"}},
			bson.M{"resource": bson.M{"db": allowedDB, "collection": ""}, "actions": bson.A{"dbStats", "collStats", "indexStats", "listCollections", "find"}},
			bson.M{"resource": bson.M{"db": allowedDB, "collection": "system.profile"}, "actions": bson.A{"find"}},
		}},
		{Key: "roles", Value: bson.A{}},
	}).Err())
	require.NoError(t, adminDB.RunCommand(ctx, bson.D{
		{Key: "createUser", Value: user},
		{Key: "pwd", Value: password},
		{Key: "roles", Value: bson.A{role}},
	}).Err())

	t.Cleanup(func() {
		ctx := context.Background()
		adminDB.RunCommand(ctx, bson.D{{Key: "dropUser", Value: user}})
		adminDB.RunCommand(ctx, bson.D{{Key: "dropRole", Value: role}})
		admin.Database(allowedDB).Drop(ctx) //nolint:errcheck
		admin.Database(deniedDB).Drop(ctx)  //nolint:errcheck
	})

	userURI := func(host string) string {
		return "mongodb://" + user + ":" + password + "@" + host + "/admin"
	}

	t.Run("direct", func(t *testing.T) {
		client := proxyTestClient(ctx, t, userURI(addr))

		dbs, err := databases(ctx, client, promslog.NewNopLogger(), nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{allowedDB}, dbs)
	})

	t.Run("authorizedDatabases rejected", func(t *testing.T) {
		proxy := startStrictProxy(ctx, t, addr, rejectArgs("listDatabases", "authorizedDatabases"))
		client := proxyTestClient(ctx, t, userURI(proxy.addr))

		dbs, err := databases(ctx, client, promslog.NewNopLogger(), nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{allowedDB}, dbs)
		assert.Positive(t, proxy.rejected.Load())

		count, complete, err := nonSystemCollectionsCount(ctx, client, promslog.NewNopLogger(), nil)
		require.NoError(t, err)
		assert.True(t, complete)
		assert.Equal(t, 2, count)
	})

	// Without connectionStatus the full list is returned, but collection discovery
	// must skip the databases the user isn't authorized on.
	t.Run("authorizedDatabases and connectionStatus rejected", func(t *testing.T) {
		proxy := startStrictProxy(ctx, t, addr, rejectAny(
			rejectArgs("listDatabases", "authorizedDatabases"),
			rejectArgs("connectionStatus"),
		))
		client := proxyTestClient(ctx, t, userURI(proxy.addr))

		dbs, err := databases(ctx, client, promslog.NewNopLogger(), nil, systemDBs)
		require.NoError(t, err)
		assert.Contains(t, dbs, deniedDB)

		namespaces, err := listAllCollections(ctx, client, promslog.NewNopLogger(), nil, systemDBs, true)
		require.NoError(t, err)
		assert.Equal(t, map[string][]string{allowedDB: {"c1", "c2"}}, namespaces)

		count, complete, err := nonSystemCollectionsCount(ctx, client, promslog.NewNopLogger(), nil)
		require.NoError(t, err)
		assert.False(t, complete, "the denied database must be reported as skipped")
		assert.Equal(t, 2, count)

		// admin is listed first and is unauthorized; the allowed database must still be collected.
		c := newProfileCollector(ctx, client, promslog.New(&promslog.Config{}), false, labelsGetterMock{}, 30)
		expected := strings.NewReader(`
	# HELP mongodb_profile_slow_query_count profile_slow_query.count
	# TYPE mongodb_profile_slow_query_count counter
	mongodb_profile_slow_query_count{database="` + allowedDB + `"} 0` + "\n")
		assert.NoError(t, testutil.CollectAndCompare(c, expected, "mongodb_profile_slow_query_count"))

		// dbstats must skip the unauthorized databases without logging an error for each one.
		var logs bytes.Buffer
		dbstats := newDBStatsCollector(ctx, client, promslog.New(&promslog.Config{Writer: &logs}), false, labelsGetterMock{}, nil, false)
		expected = strings.NewReader(`
	# HELP mongodb_dbstats_collections dbstats.collections
	# TYPE mongodb_dbstats_collections untyped
	mongodb_dbstats_collections{database="` + allowedDB + `"} 2` + "\n")
		require.NoError(t, testutil.CollectAndCompare(dbstats, expected, "mongodb_dbstats_collections"))
		assert.NotContains(t, logs.String(), "level=ERROR")
	})
}

// A privilege on all databases ({db: "", collection: ""}) doesn't cover local and config. The
// fallback must return the same list as the server.
//
//nolint:paralleltest
func TestListDatabasesAllDatabasesPrivilege(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	const (
		role     = "privtestAllDBsRole"
		user     = "privtestalldbs"
		password = "privtestalldbs"
	)

	addr := net.JoinHostPort("127.0.0.1", tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"))
	admin := proxyTestClient(ctx, t, "mongodb://admin:admin@"+addr)

	adminDB := admin.Database("admin")
	adminDB.RunCommand(ctx, bson.D{{Key: "dropUser", Value: user}})
	adminDB.RunCommand(ctx, bson.D{{Key: "dropRole", Value: role}})
	require.NoError(t, adminDB.RunCommand(ctx, bson.D{
		{Key: "createRole", Value: role},
		{Key: "privileges", Value: bson.A{
			bson.M{"resource": bson.M{"cluster": true}, "actions": bson.A{"listDatabases"}},
			bson.M{"resource": bson.M{"db": "", "collection": ""}, "actions": bson.A{"find"}},
		}},
		{Key: "roles", Value: bson.A{}},
	}).Err())
	require.NoError(t, adminDB.RunCommand(ctx, bson.D{
		{Key: "createUser", Value: user},
		{Key: "pwd", Value: password},
		{Key: "roles", Value: bson.A{role}},
	}).Err())

	t.Cleanup(func() {
		ctx := context.Background()
		adminDB.RunCommand(ctx, bson.D{{Key: "dropUser", Value: user}})
		adminDB.RunCommand(ctx, bson.D{{Key: "dropRole", Value: role}})
	})

	userURI := func(host string) string {
		return "mongodb://" + user + ":" + password + "@" + host + "/admin"
	}

	want, err := databases(ctx, proxyTestClient(ctx, t, userURI(addr)), promslog.NewNopLogger(), nil, nil)
	require.NoError(t, err)
	assert.NotContains(t, want, "local")
	assert.NotContains(t, want, "config")

	proxy := startStrictProxy(ctx, t, addr, rejectArgs("listDatabases", "authorizedDatabases"))

	got, err := databases(ctx, proxyTestClient(ctx, t, userURI(proxy.addr)), promslog.NewNopLogger(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Positive(t, proxy.rejected.Load())
}

//nolint:funlen
func TestDatabasesFromPrivileges(t *testing.T) {
	t.Parallel()

	str := func(s string) *string { return &s }

	all := []string{"admin", "app1", "app2", "config", "local"}

	tests := []struct {
		name      string
		resources []privilegeResource
		want      []string
	}{
		{
			name:      "no privileges",
			resources: nil,
			want:      []string{},
		},
		{
			name: "cluster privilege only",
			resources: []privilegeResource{
				{}, // {cluster: true}
			},
			want: []string{},
		},
		{
			name: "database and collection privileges",
			resources: []privilegeResource{
				{},
				{DB: str("app1"), Collection: str("")},
				{DB: str("app2"), Collection: str("c1")},
				{DB: str("app2"), Collection: str("c2")},
			},
			want: []string{"app1", "app2"},
		},
		{
			name: "system buckets in one database",
			resources: []privilegeResource{
				{DB: str("app1"), SystemBuckets: str("")},
			},
			want: []string{"app1"},
		},
		{
			name:      "all databases excludes local and config",
			resources: []privilegeResource{{DB: str(""), Collection: str("")}},
			want:      []string{"admin", "app1", "app2"},
		},
		{
			name: "all databases plus explicit local",
			resources: []privilegeResource{
				{DB: str(""), Collection: str("")},
				{DB: str("local"), Collection: str("")},
			},
			want: []string{"admin", "app1", "app2", "local"},
		},
		{
			name:      "one collection name in all databases",
			resources: []privilegeResource{{DB: str("app1")}, {DB: str(""), Collection: str("system.profile")}},
			want:      all,
		},
		{
			name:      "system buckets in any database",
			resources: []privilegeResource{{SystemBuckets: str("")}},
			want:      all,
		},
		{
			name:      "any resource",
			resources: []privilegeResource{{AnyResource: true}},
			want:      all,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			access := databasesFromPrivileges(tc.resources)

			got := []string{}
			for _, name := range all {
				if access.allows(name) {
					got = append(got, name)
				}
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFilterDatabaseNames(t *testing.T) {
	t.Parallel()

	all := []string{"admin", "app1", "app2", "config", "local"}

	tests := []struct {
		name       string
		namespaces []string
		exclude    []string
		want       []string
	}{
		{name: "no filters", want: all},
		{name: "exclude system databases", exclude: systemDBs, want: []string{"app1", "app2"}},
		{name: "namespaces", namespaces: []string{"app2.coll.with.dots", "", "admin"}, want: []string{"admin", "app2"}},
		{name: "namespaces and exclude", namespaces: []string{"app1.c", "admin"}, exclude: systemDBs, want: []string{"app1"}},
		{name: "unknown namespace", namespaces: []string{"nope.c"}, want: []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, filterDatabaseNames(all, tc.namespaces, tc.exclude))
		})
	}
}

func TestSplitNamespace(t *testing.T) {
	testCases := []struct {
		namespace      string
		wantDatabase   string
		wantCollection string
	}{
		{
			namespace:      "db.collection",
			wantDatabase:   "db",
			wantCollection: "collection",
		},
		{
			namespace:      "db.this.is.a.valid.collection.name",
			wantDatabase:   "db",
			wantCollection: "this.is.a.valid.collection.name",
		},
	}

	for _, tc := range testCases {
		db, coll := splitNamespace(tc.namespace)
		assert.Equal(t, tc.wantDatabase, db)
		assert.Equal(t, tc.wantCollection, coll)
	}
}

func TestSetShardLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  bson.M
		want map[string]string
	}{
		{
			name: "shard set",
			doc:  bson.M{"shard": "shard-1"},
			want: map[string]string{"database": "testdb", "shard": "shard-1"},
		},
		{
			name: "empty shard",
			doc:  bson.M{"shard": ""},
			want: map[string]string{"database": "testdb", "shard": ""},
		},
		{
			name: "shard field absent",
			doc:  bson.M{},
			want: map[string]string{"database": "testdb", "shard": ""},
		},
		{
			name: "shard field is not a string",
			doc:  bson.M{"shard": 42},
			want: map[string]string{"database": "testdb", "shard": ""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// A shard from a previous document must not leak into this one.
			labels := map[string]string{"database": "testdb", "shard": "shard-0"}
			setShardLabel(labels, test.doc)

			require.Equal(t, test.want, labels)
		})
	}
}

//nolint:paralleltest
func TestCheckNamespacesForViews(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	setupDB(ctx, t, client)
	defer cleanupDB(ctx, client)

	t.Run("Views in provided collection list (should fail)", func(t *testing.T) {
		_, err := checkNamespacesForViews(ctx, client, promslog.NewNopLogger(), []string{"testdb01.col01", "testdb01.system.views", "testdb01.view01"})
		assert.EqualError(t, err, "namespace testdb01.view01 is a view and cannot be used for collstats/indexstats")
	})

	t.Run("No Views in provided collection list", func(t *testing.T) {
		filtered, err := checkNamespacesForViews(ctx, client, promslog.NewNopLogger(), []string{"testdb01.col01", "testdb01.system.views"})
		assert.NoError(t, err)
		assert.Equal(t, []string{"testdb01.col01", "testdb01.system.views"}, filtered)
	})
}

func TestIsUnauthorizedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "server Unauthorized",
			err:  mongo.CommandError{Code: 13, Name: "Unauthorized", Message: "not authorized on app to execute command { dbStats: 1 }"},
			want: true,
		},
		{
			name: "Atlas not authorized",
			err:  mongo.CommandError{Code: 8000, Name: "AtlasError", Message: "(Unauthorized) not authorized on local to execute command { dbStats: 1 }"},
			want: true,
		},
		{
			name: "Atlas action not allowed",
			err:  mongo.CommandError{Code: 8000, Name: "AtlasError", Message: "user is not allowed to do action [find] on [app.system.profile]"},
			want: true,
		},
		{
			name: "Atlas unallowed argument",
			err:  mongo.CommandError{Code: 8000, Name: "AtlasError", Message: "Unallowed argument in listDatabases command: authorizedDatabases"},
			want: false,
		},
		{
			name: "Atlas tier restriction",
			err:  mongo.CommandError{Code: 8000, Name: "AtlasError", Message: "{getParameter: 1, featureCompatibilityVersion: \"\"} not allowed in this atlas tier"},
			want: false,
		},
		{
			name: "other server error",
			err:  mongo.CommandError{Code: 26, Name: "NamespaceNotFound", Message: "ns does not exist"},
			want: false,
		},
		{
			name: "network error",
			err:  mongo.CommandError{Labels: []string{"NetworkError"}, Message: "socket was unexpectedly closed: EOF"},
			want: false,
		},
		{
			name: "not a command error",
			err:  context.DeadlineExceeded,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isUnauthorizedError(tt.err))
		})
	}
}
