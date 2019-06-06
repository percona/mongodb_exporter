package mongod

import (
	"context"
	"testing"

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

	if len(topStatus.TopStats) < len(collections) {
		t.Errorf("All database collections were not loaded, expected: %v, got: %v", len(collections), len(topStatus.TopStats))
	}

	for cid := range collections {
		if _, ok := topStatus.TopStats[collections[cid]]; !ok {
			t.Errorf("Database collection is missing, %v", collections[cid])
		}
	}
}
