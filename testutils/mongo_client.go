package testutils

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MustGetConnectedReplSetClient return mongo.Client instance connected to server started in replicaSet mode.
func MustGetConnectedReplSetClient(t *testing.T, ctx context.Context) *mongo.Client {
	opts := options.Client().
		ApplyURI("mongodb://127.0.0.1:27019/admin").
		SetReplicaSet("rs0").
		SetDirect(true).SetServerSelectionTimeout(time.Second)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		t.Fatal(errors.Wrap(err, "Couldn't connect to MongoDB instance"))
	}

	return client
}

// MustGetConnectedMongodClient return mongo.Client instance connected to server started in single mode.
func MustGetConnectedMongodClient(t *testing.T, ctx context.Context) *mongo.Client {
	opts := options.Client().
		ApplyURI("mongodb://127.0.0.1:27017/admin").
		SetDirect(true).SetServerSelectionTimeout(time.Second)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		t.Fatal(errors.Wrap(err, "Couldn't connect to MongoDB instance"))
	}

	return client
}
