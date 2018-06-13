package mongod

import (
	"testing"

	"gopkg.in/mgo.v2/bson"
)

func Test_ParserTopStatus(t *testing.T) {
	data := LoadFixture("top_status.bson")
	collections := []string{
		"admin.system.roles",
		"admin.system.version",
		"dummy.collection",
		"dummy.users",
		"local.oplog.rs",
		"local.startup_log",
		"local.system.replset",
	}

	topStatus := &TopStatus{}
	loadTopStatusFromBson(data, topStatus)

	topStats := topStatus.TopStats["dummy.users"]

	if len(topStatus.TopStats) != len(collections) {
		t.Error("All database collections were not loaded")
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

func loadTopStatusFromBson(data []byte, status *TopStatus) {
	err := bson.Unmarshal(data, status)
	if err != nil {
		panic(err)
	}
}
