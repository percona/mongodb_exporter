// mongodb_exporter
// Copyright (C) 2022 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/percona/mongodb_exporter/internal/tu"
)

type ReplicasetConfig struct {
	Config RSConfig `bson:"config"`
}

type RSConfig struct {
	ID                                 string `bson:"_id"`
	Version                            int    `bson:"version"`
	ProtocolVersion                    int    `bson:"protocolVersion"`
	WriteConcernMajorityJournalDefault bool   `bson:"writeConcernMajorityJournalDefault"`
	Members                            []struct {
		ID           int      `bson:"_id"`
		Host         string   `bson:"host"`
		ArbiterOnly  bool     `bson:"arbiterOnly"`
		BuildIndexes bool     `bson:"buildIndexes"`
		Hidden       bool     `bson:"hidden"`
		Priority     int      `bson:"priority"`
		Tags         struct{} `bson:"tags"`
		SlaveDelay   int      `bson:"slaveDelay"`
		Votes        int      `bson:"votes"`
	} `bson:"members"`
	Settings struct {
		ChainingAllowed            bool     `bson:"chainingAllowed"`
		HeartbeatIntervalMillis    int      `bson:"heartbeatIntervalMillis"`
		HeartbeatTimeoutSecs       int      `bson:"heartbeatTimeoutSecs"`
		ElectionTimeoutMillis      int      `bson:"electionTimeoutMillis"`
		CatchUpTimeoutMillis       int      `bson:"catchUpTimeoutMillis"`
		CatchUpTakeoverDelayMillis int      `bson:"catchUpTakeoverDelayMillis"`
		GetLastErrorModes          struct{} `bson:"getLastErrorModes"`
		GetLastErrorDefaults       struct {
			W        int `bson:"w"`
			Wtimeout int `bson:"wtimeout"`
		} `bson:"getLastErrorDefaults"`
		ReplicaSetID primitive.ObjectID `bson:"replicaSetId"`
	} `bson:"settings"`
}

func TestSecondaryLag(t *testing.T) {
	t.Skip("This is failing in GitHub actions. Cannot make secondary to lag behind")
	secondsBehind := 3
	sleep := 2
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration((secondsBehind*2)+sleep)*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	var rsConf, rsConfOld ReplicasetConfig
	var gg interface{}

	res := client.Database("admin").RunCommand(ctx, primitive.M{"replSetGetConfig": 1})
	require.NoError(t, res.Err())

	err := res.Decode(&gg) // To restore config after test
	assert.NoError(t, err)

	err = res.Decode(&rsConf)
	assert.NoError(t, err)

	rsConf.Config.Members[1].Priority = 0
	rsConf.Config.Members[1].Hidden = true
	rsConf.Config.Members[1].SlaveDelay = secondsBehind
	rsConf.Config.Version++

	var replSetReconfig struct {
		OK int `bson:"ok"`
	}
	err = client.Database("admin").RunCommand(ctx, primitive.M{"replSetReconfig": rsConf.Config}).Decode(&replSetReconfig)
	assert.NoError(t, err)

	res = client.Database("admin").RunCommand(ctx, primitive.M{"replSetGetConfig": 1})
	require.NoError(t, res.Err())

	// Generate documents so oplog is forced to have operations and the lag becomes real, otherwise
	// primary and secondary oplogs are the same. Generate more than one doc to ensure oplog is updated
	// quickly for the test.
	for i := 0; i < 100; i++ {
		_, err = client.Database("test").Collection("testc1").InsertOne(ctx, bson.M{"s": 1})
		require.NoError(t, err)
		time.Sleep(20 * time.Millisecond)
	}
	err = client.Database("test").Drop(ctx)
	assert.NoError(t, err)

	err = res.Decode(&rsConfOld) // To restore config after test
	assert.NoError(t, err)

	msclient := tu.TestClient(ctx, tu.MongoDBS1Secondary1Port, t)
	var m bson.M

	cmd := bson.D{{Key: "getDiagnosticData", Value: "1"}}
	res = msclient.Database("admin").RunCommand(ctx, cmd)

	err = res.Decode(&m)
	assert.NoError(t, err)

	m, _ = m["data"].(bson.M)
	metrics := replSetMetrics(m)
	var lag prometheus.Metric
	for _, m := range metrics {
		if strings.HasPrefix(m.Desc().String(), `Desc{fqName: "mongodb_mongod_replset_member_replication_lag"`) {
			lag = m

			break
		}
	}

	metric := &dto.Metric{}
	err = lag.Write(metric)
	assert.NoError(t, err)
	// Secondary is not exactly secondsBehind behind master
	assert.True(t, *metric.Gauge.Value > 0)

	rsConfOld.Config.Version = rsConf.Config.Version + 1
	err = client.Database("admin").RunCommand(ctx, primitive.M{"replSetReconfig": rsConfOld.Config}).Decode(&replSetReconfig)
	assert.NoError(t, err)
}
