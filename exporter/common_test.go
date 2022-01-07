package exporter

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:gochecknoglobals
var (
	testDBs   = []string{"testdb01", "testdb02"}
	testColls = []string{"col01", "col02", "colxx", "colyy"}
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
}

func cleanupDB(ctx context.Context, client *mongo.Client) {
	for _, dbname := range testDBs {
		client.Database(dbname).Drop(ctx) //nolint:errcheck
	}
}

//nolint:paralleltest
func TestListDatabases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
		colls, err := listCollections(ctx, client, testDBs[0], inNameSpaces)
		sort.Strings(colls)

		assert.NoError(t, err)
		assert.Equal(t, want, colls)
	})

	t.Run("With namespaces list", func(t *testing.T) {
		// Advanced filtering test
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy"},
			"testdb02": {"col01", "col02"},
		}
		// List all collections in testdb01 (inDBs[0]) but only col01 and col02 from testdb02.
		filterInNameSpaces := []string{testDBs[0], testDBs[1] + ".col01", testDBs[1] + ".col02"}
		namespaces, err := listAllCollections(ctx, client, filterInNameSpaces, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Empty namespaces list", func(t *testing.T) {
		wantNS := map[string][]string{
			"testdb01": {"col01", "col02", "colxx", "colyy"},
			"testdb02": {"col01", "col02", "colxx", "colyy"},
		}
		namespaces, err := listAllCollections(ctx, client, nil, systemDBs)
		assert.NoError(t, err)
		assert.Equal(t, wantNS, namespaces)
	})

	t.Run("Count basic", func(t *testing.T) {
		count, err := nonSystemCollectionsCount(ctx, client, nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, 8, count)
	})

	t.Run("Filtered count", func(t *testing.T) {
		count, err := nonSystemCollectionsCount(ctx, client, nil, []string{testDBs[0] + ".col0", testDBs[0] + ".colx"})
		assert.NoError(t, err)
		assert.Equal(t, 6, count)
	})
}
