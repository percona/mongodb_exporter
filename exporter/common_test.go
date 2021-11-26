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

	databases := []string{"testdb01", "testdb02"}
	collections := []string{"col01", "col02", "colxx", "colyy"}

	defer func() {
		for _, dbname := range databases {
			client.Database(dbname).Drop(ctx) //nolint:errcheck
		}
	}()

	for _, dbname := range databases {
		for _, coll := range collections {
			for j := 0; j < 10; j++ {
				_, err := client.Database(dbname).Collection(coll).InsertOne(ctx, bson.M{"f1": j, "f2": "2"})
				assert.NoError(t, err)
			}
		}
	}

	want := []string{"col01", "col02", "colxx"}
	collections, err := listCollections(ctx, client, []string{"col0", "colx"}, databases[0])
	sort.Strings(collections)

	assert.NoError(t, err)
	assert.Equal(t, want, collections)

	count, err := allCollectionsCount(ctx, client, nil)
	assert.NoError(t, err)
	assert.True(t, count > 8)

	count, err = allCollectionsCount(ctx, client, []string{"col0", "colx"})
	assert.NoError(t, err)
	assert.Equal(t, 6, count)
}
