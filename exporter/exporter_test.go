package exporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getTestClient(t *testing.T) *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client()) //.ApplyURI("mongodb://127.0.0.1:27017"))
	require.NoError(t, err)

	t.Cleanup(func() { client.Disconnect(ctx) })

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	return client
}
