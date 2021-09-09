package exporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/percona/mongodb_exporter/internal/tu"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	databasesCount   = 1000
	collectionsCount = 10
	documentsCount   = 1000
)

func prepare(ctx context.Context, client *mongo.Client, tb testing.TB) error {
	tb.Helper()
	for i := 0; i < databasesCount; i++ {
		dbname := fmt.Sprintf("benchdb_%06d", i)
		database := client.Database(dbname)
		tb.Logf("Using database %s", dbname)

		for j := 0; j < collectionsCount; j++ {
			collName := fmt.Sprintf("collection_%06d", j)

			docs := make([]interface{}, 0, documentsCount)
			for k := 0; k < documentsCount; k++ {
				n := i*databasesCount + j*collectionsCount + k
				docs = append(docs, primitive.M{"f1": n})
			}

			if _, err := database.Collection(collName).InsertMany(ctx, docs); err != nil {
				return errors.Wrap(err, "cannot prepare benchamark data")
			}
		}
	}

	return nil
}

func cleanup(ctx context.Context, client *mongo.Client, tb testing.TB) {
	tb.Helper()
	for i := 0; i < databasesCount; i++ {
		dbname := fmt.Sprintf("benchdb_%06d", i)
		database := client.Database(dbname)
		database.Drop(ctx) //nolint
	}
}

func BenchmarkDbStats(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, b)

	b.Logf("Preparing benchmar data")

	err := prepare(ctx, client, b)
	require.NoError(b, err)
	defer cleanup(ctx, client, b)

	collector := &dbstatsCollector{
		ctx:            ctx,
		client:         client,
		compatibleMode: false,
		logger:         logrus.New(),
		topologyInfo:   labelsGetterMock{},
	}

	ch := make(chan prometheus.Metric, 1000)
	var count int

	// drain the channel
	go func() {
		for _ = range ch {
			count++
		}
	}()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		collector.Collect(ch)
	}

	close(ch)
}
