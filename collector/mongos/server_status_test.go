package collector_mongos

import (
	"io/ioutil"
	"testing"

	"gopkg.in/mgo.v2/bson"
)

func Test_ParserServerStatus(t *testing.T) {
	data, err := ioutil.ReadFile("../fixtures/server_status.bson")
	if err != nil {
		t.Fatal(err)
	}

	serverStatus := &ServerStatus{}
	loadServerStatusFromBson(data, serverStatus)

	if serverStatus.Asserts == nil {
		t.Error("Asserts group was not loaded")
	}

	if serverStatus.Connections == nil {
		t.Error("Connections group was not loaded")
	}

	if serverStatus.ExtraInfo == nil {
		t.Error("ExtraInfo group was not loaded")
	}

	if serverStatus.Network == nil {
		t.Error("Network group was not loaded")
	}

	if serverStatus.Opcounters == nil {
		t.Error("Opcounters group was not loaded")
	}

	if serverStatus.Mem == nil {
		t.Error("Mem group was not loaded")
	}

	if serverStatus.Connections == nil {
		t.Error("Connections group was not loaded")
	}
}

func loadServerStatusFromBson(data []byte, status *ServerStatus) {
	err := bson.Unmarshal(data, status)
	if err != nil {
		panic(err)
	}
}
