// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongod

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

	if serverStatus.Version != "2.6.7" {
		t.Errorf("Server version incorrect: %s", serverStatus.Version)
	}

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
