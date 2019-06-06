package mongod

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Test_ParserTopStatus(t *testing.T) {
	topStatus := &TopStatus{}

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.TODO())
	// TODO: Not working as of "note" field in mongodb result...
	err = client.Database("admin").RunCommand(context.TODO(), bson.D{{"top", 1}}).Decode(&topStatus)
	if err != nil {
		t.Fatal(err)
	}

	collections := []string{
		"admin.system.roles",
		"admin.system.version",
		"admin.system.users",
		"admin.system.sessions",
		"local.startup_log",
		"local.system.replset",
	}

	topStats := topStatus.TopStats["dummy.users"]

	if len(topStatus.TopStats) != len(collections) {
		t.Errorf("All database collections were not loaded, expected: %v, got: %v", len(collections), len(topStatus.TopStats))
	}

	for cid := range collections {
		if _, ok := topStatus.TopStats[collections[cid]]; !ok {
			t.Error("Database collection is missing")
		}
	}

	if topStats.Total.Time != 1095531 {
		t.Error("Wrong total operation time value for dummy user collection")
	}
	if topStats.Total.Count != 17428 {
		t.Error("Wrong total operation count value for dummy user collection")
	}

	if topStats.ReadLock.Time != 267953 {
		t.Error("Wrong read lock operation time value for dummy user collection")
	}
	if topStats.ReadLock.Count != 17420 {
		t.Error("Wrong read lock operation count value for dummy user collection")
	}

	if topStats.WriteLock.Time != 827578 {
		t.Error("Wrong write lock operation time value for dummy user collection")
	}
	if topStats.WriteLock.Count != 8 {
		t.Error("Wrong write lock operation count value for dummy user collection")
	}

	if topStats.Queries.Time != 899 {
		t.Error("Wrong queries operation time value for dummy user collection")
	}
	if topStats.Queries.Count != 10 {
		t.Error("Wrong queries operation count value for dummy user collection")
	}

	if topStats.GetMore.Time != 0 {
		t.Error("Wrong get more operation time value for dummy user collection")
	}
	if topStats.GetMore.Count != 0 {
		t.Error("Wrong get more operation count value for dummy user collection")
	}

	if topStats.Insert.Time != 826929 {
		t.Error("Wrong insert operation time value for dummy user collection")
	}
	if topStats.Insert.Count != 5 {
		t.Error("Wrong insert operation count value for dummy user collection")
	}

	if topStats.Update.Time != 456 {
		t.Error("Wrong update operation time value for dummy user collection")
	}
	if topStats.Update.Count != 2 {
		t.Error("Wrong update operation count value for dummy user collection")
	}

	if topStats.Remove.Time != 193 {
		t.Error("Wrong remove operation time value for dummy user collection")
	}
	if topStats.Remove.Count != 1 {
		t.Error("Wrong remove operation count value for dummy user collection")
	}

	if topStats.Commands.Time != 0 {
		t.Error("Wrong commands operation time value for dummy user collection")
	}
	if topStats.Commands.Count != 0 {
		t.Error("Wrong commands operation count value for dummy user collection")
	}
}
