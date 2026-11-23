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
	"context"
	"net"
	"sort"
	"testing"
	"time"

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
		allDBs, err := databases(ctx, client, nil, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, want, allDBs)
	})

	t.Run("One collection in list", func(t *testing.T) {
		want := []string{"testdb01"}
		allDBs, err := databases(ctx, client, []string{"testdb01.col1"}, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, want, allDBs)
	})

	t.Run("Multiple namespaces in list", func(t *testing.T) {
		want := []string{"testdb01", "testdb02"}
		allDBs, err := databases(ctx, client, []string{"testdb01", "testdb02.col2", "testdb02.col1"}, systemDBs)
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
		allDBs, err := databases(ctx, client, nil, nil)
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
		namespaces, err := listAllCollections(ctx, client, filterInNameSpaces, systemDBs, true)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Empty namespaces list", func(t *testing.T) {
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy", "system.views"},
			"testdb02": {"col01", "col02", "colxx", "colyy"},
		}
		namespaces, err := listAllCollections(ctx, client, nil, systemDBs, true)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Collections with views", func(t *testing.T) {
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy", "system.views", "view01", "view02"},
			"testdb02": {"col01", "col02", "colxx", "colyy"},
		}
		namespaces, err := listAllCollections(ctx, client, nil, systemDBs, false)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Count basic", func(t *testing.T) {
		count, err := nonSystemCollectionsCount(ctx, client, nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, 9, count)
	})

	t.Run("Filtered count", func(t *testing.T) {
		count, err := nonSystemCollectionsCount(ctx, client, nil, []string{testDBs[0] + ".col0", testDBs[0] + ".colx"})
		assert.NoError(t, err)
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

// Some managed services reject listDatabases arguments outside an allow-list (issue #571, #412).
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

			allDBs, err := databases(ctx, proxyClient, nil, systemDBs)
			require.NoError(t, err)
			assert.Equal(t, testDBs, allDBs)

			allDBs, err = databases(ctx, proxyClient, []string{"testdb02.col01"}, systemDBs)
			require.NoError(t, err)
			assert.Equal(t, []string{"testdb02"}, allDBs)

			assert.Positive(t, proxy.rejected.Load(), "the proxy should have rejected the command")
		})
	}
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

		dbs, err := databases(ctx, client, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{allowedDB}, dbs)
	})

	t.Run("authorizedDatabases rejected", func(t *testing.T) {
		proxy := startStrictProxy(ctx, t, addr, rejectArgs("listDatabases", "authorizedDatabases"))
		client := proxyTestClient(ctx, t, userURI(proxy.addr))

		dbs, err := databases(ctx, client, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{allowedDB}, dbs)
		assert.Positive(t, proxy.rejected.Load())

		count, err := nonSystemCollectionsCount(ctx, client, nil, nil)
		require.NoError(t, err)
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

		dbs, err := databases(ctx, client, nil, systemDBs)
		require.NoError(t, err)
		assert.Contains(t, dbs, deniedDB)

		namespaces, err := listAllCollections(ctx, client, nil, systemDBs, true)
		require.NoError(t, err)
		assert.Equal(t, map[string][]string{allowedDB: {"c1", "c2"}}, namespaces)

		count, err := nonSystemCollectionsCount(ctx, client, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

//nolint:funlen
func TestDatabasesFromPrivileges(t *testing.T) {
	t.Parallel()

	str := func(s string) *string { return &s }

	tests := []struct {
		name      string
		resources []privilegeResource
		want      map[string]struct{}
		wantAll   bool
	}{
		{
			name:      "no privileges",
			resources: nil,
			want:      map[string]struct{}{},
		},
		{
			name: "cluster privilege only",
			resources: []privilegeResource{
				{}, // {cluster: true}
			},
			want: map[string]struct{}{},
		},
		{
			name: "database and collection privileges",
			resources: []privilegeResource{
				{},
				{DB: str("app1"), SystemBuckets: nil},
				{DB: str("app2")},
				{DB: str("app2")},
			},
			want: map[string]struct{}{"app1": {}, "app2": {}},
		},
		{
			name: "system buckets in one database",
			resources: []privilegeResource{
				{DB: str("app1"), SystemBuckets: str("")},
			},
			want: map[string]struct{}{"app1": {}},
		},
		{
			name:      "any database",
			resources: []privilegeResource{{DB: str("app1")}, {DB: str("")}},
			wantAll:   true,
		},
		{
			name:      "system buckets in any database",
			resources: []privilegeResource{{SystemBuckets: str("")}},
			wantAll:   true,
		},
		{
			name:      "any resource",
			resources: []privilegeResource{{AnyResource: true}},
			wantAll:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, all := databasesFromPrivileges(tc.resources)
			assert.Equal(t, tc.wantAll, all)
			if !tc.wantAll {
				assert.Equal(t, tc.want, got)
			}
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
		_, err := checkNamespacesForViews(ctx, client, []string{"testdb01.col01", "testdb01.system.views", "testdb01.view01"})
		assert.EqualError(t, err, "namespace testdb01.view01 is a view and cannot be used for collstats/indexstats")
	})

	t.Run("No Views in provided collection list", func(t *testing.T) {
		filtered, err := checkNamespacesForViews(ctx, client, []string{"testdb01.col01", "testdb01.system.views"})
		assert.NoError(t, err)
		assert.Equal(t, []string{"testdb01.col01", "testdb01.system.views"}, filtered)
	})
}
