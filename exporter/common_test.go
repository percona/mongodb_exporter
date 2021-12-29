package exporter

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestListCollections(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	inDBs := []string{"testdb01", "testdb02"}
	inColls := []string{"col01", "col02", "colxx", "colyy"}

	defer func() {
		for _, dbname := range inDBs {
			client.Database(dbname).Drop(ctx) //nolint:errcheck
		}
	}()

	for _, dbname := range inDBs {
		for _, coll := range inColls {
			for j := 0; j < 10; j++ {
				_, err := client.Database(dbname).Collection(coll).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
				assert.NoError(t, err)
			}
		}
	}

	want := []string{"admin", "config", "local", "testdb01", "testdb02"}
	allDBs, err := databases(ctx, client, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, want, allDBs)

	want = []string{"col01", "col02", "colxx"}
	inNameSpaces := []string{inDBs[0] + ".col0", inDBs[0] + ".colx"}
	colls, err := listCollections(ctx, client, inDBs[0], inNameSpaces)
	sort.Strings(colls)

	assert.NoError(t, err)
	assert.Equal(t, want, colls)

	count, err := nonSystemCollectionsCount(ctx, client, nil, nil)
	assert.NoError(t, err)
	assert.True(t, count == 8)

	count, err = nonSystemCollectionsCount(ctx, client, nil, []string{inDBs[0] + ".col0", inDBs[0] + ".colx"})
	assert.NoError(t, err)
	assert.Equal(t, 6, count)
}
