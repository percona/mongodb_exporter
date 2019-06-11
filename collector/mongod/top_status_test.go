package mongod

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Test_ParserTopStatus(t *testing.T) {
	raw := &TopStatusRaw{}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.TODO())
	err = client.Database("admin").RunCommand(context.TODO(), bson.D{{"top", 1}}).Decode(&raw)
	if err != nil {
		t.Fatal(err)
	}

	topStatus := raw.TopStatus()

	collections := []string{
		"admin.system.roles",
		"admin.system.version",
		"local.startup_log",
		"local.system.replset",
	}

	assert.True(t, len(collections) <= len(topStatus.TopStats),
		"expected more than %d collections, got %d", len(collections), len(topStatus.TopStats))
	for _, col := range collections {
		assert.Contains(t, topStatus.TopStats, col)
		stats := topStatus.TopStats[col]
		assert.NotZero(t, stats.Total.Time, "%s: %+v", col, stats)
		assert.NotZero(t, stats.Total.Count, "%s: %+v", col, stats)
	}
}
