package collector_mongod

import (
	"io/ioutil"
	"testing"

	"gopkg.in/mgo.v2/bson"
)

func TestParserServerStatus(t *testing.T) {
	data, err := ioutil.ReadFile("../fixtures/server_status.bson")
	if err != nil {
		t.Fatal(err)
	}

	serverStatus := &ServerStatus{}
	loadServerStatusFromBson(data, serverStatus)

	if serverStatus.Asserts == nil {
		t.Error("Asserts group was not loaded")
	}

	if serverStatus.Dur == nil {
		t.Error("Dur group was not loaded")
	}

	if serverStatus.BackgroundFlushing == nil {
		t.Error("BackgroundFlushing group was not loaded")
	}

	if serverStatus.Connections == nil {
		t.Error("Connections group was not loaded")
	}

	if serverStatus.ExtraInfo == nil {
		t.Error("ExtraInfo group was not loaded")
	}

	if serverStatus.GlobalLock == nil {
		t.Error("GlobalLock group was not loaded")
	}

	if serverStatus.Network == nil {
		t.Error("Network group was not loaded")
	}

	if serverStatus.Opcounters == nil {
		t.Error("Opcounters group was not loaded")
	}

	if serverStatus.OpcountersRepl == nil {
		t.Error("OpcountersRepl group was not loaded")
	}

	if serverStatus.Mem == nil {
		t.Error("Mem group was not loaded")
	}

	if serverStatus.Connections == nil {
		t.Error("Connections group was not loaded")
	}

	if serverStatus.Locks == nil {
		t.Error("Locks group was not loaded")
	}

	if serverStatus.Metrics.Document.Deleted != 45726 {
		t.Error("Metrics group was not loaded correctly")
	}
}

func loadServerStatusFromBson(data []byte, status *ServerStatus) {
	err := bson.Unmarshal(data, status)
	if err != nil {
		panic(err)
	}
}
