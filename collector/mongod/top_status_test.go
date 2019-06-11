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
		"config.system.sessions",
		"local.startup_log",
		"local.system.replset",
	}

	assert.Len(t, topStatus.TopStats, len(collections))
	for col, stats := range topStatus.TopStats {
		assert.Contains(t, collections, col)
		assert.NotZero(t, stats.Total.Time, "%s: %+v", col, stats)
		assert.NotZero(t, stats.Total.Count, "%s: %+v", col, stats)
	}
}
