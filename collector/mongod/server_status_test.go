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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/percona/mongodb_exporter/testutils"
)

func TestParserServerStatus(t *testing.T) {

	serverStatus := &ServerStatus{}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.TODO())

	err = client.Database("admin").RunCommand(context.TODO(), bson.D{
		{Key: "serverStatus", Value: 1},
		{Key: "recordStats", Value: 0},
		{Key: "opLatencies", Value: bson.M{"histograms": true}},
	}).Decode(serverStatus)
	if err != nil {
		t.Fatal(err)
	}

	if serverStatus.Version == "" {
		t.Errorf("Server version incorrect")
	}

	if serverStatus.Asserts == nil {
		t.Error("Asserts group was not loaded")
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

	if serverStatus.Metrics.Document == nil {
		t.Error("Metrics group was not loaded correctly")
	}
}

func TestGetServerStatusDecodesFine(t *testing.T) {
	// setup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defaultClient := testutils.MustGetConnectedMongodClient(ctx, t)
	defer defaultClient.Disconnect(ctx)

	// run
	statusDefault := GetServerStatus(defaultClient)

	// test
	assert.NotNil(t, statusDefault)
	assert.Equal(t, 1.0, statusDefault.Ok)
}
