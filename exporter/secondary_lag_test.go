package exporter

import (
	"context"
	"testing"
	"time"

	"github.com/percona/mongodb_exporter/internal/tu"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		ID           int    `bson:"_id"`
		Host         string `bson:"host"`
		ArbiterOnly  bool   `bson:"arbiterOnly"`
		BuildIndexes bool   `bson:"buildIndexes"`
		Hidden       bool   `bson:"hidden"`
		Priority     int    `bson:"priority"`
		Tags         struct {
		} `bson:"tags"`
		SlaveDelay int `bson:"slaveDelay"`
		Votes      int `bson:"votes"`
	} `bson:"members"`
	Settings struct {
		ChainingAllowed            bool `bson:"chainingAllowed"`
		HeartbeatIntervalMillis    int  `bson:"heartbeatIntervalMillis"`
		HeartbeatTimeoutSecs       int  `bson:"heartbeatTimeoutSecs"`
		ElectionTimeoutMillis      int  `bson:"electionTimeoutMillis"`
		CatchUpTimeoutMillis       int  `bson:"catchUpTimeoutMillis"`
		CatchUpTakeoverDelayMillis int  `bson:"catchUpTakeoverDelayMillis"`
		GetLastErrorModes          struct {
		} `bson:"getLastErrorModes"`
		GetLastErrorDefaults struct {
			W        int `bson:"w"`
			Wtimeout int `bson:"wtimeout"`
		} `bson:"getLastErrorDefaults"`
		ReplicaSetID primitive.ObjectID `bson:"replicaSetId"`
	} `bson:"settings"`
}

func TestSecondaryLag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	var rsConf, rsConfOld ReplicasetConfig

	res := client.Database("admin").RunCommand(ctx, primitive.M{"replSetGetConfig": 1})
	require.NoError(t, res.Err())

	err := res.Decode(&rsConfOld) // To restore config after test
	assert.NoError(t, err)

	err = res.Decode(&rsConf)
	assert.NoError(t, err)

	secondsBehind := int(3600)
	rsConf.Config.Members[1].Priority = 0
	rsConf.Config.Members[1].Hidden = true
	rsConf.Config.Members[1].SlaveDelay = secondsBehind
	rsConf.Config.Version++

	var replSetReconfig struct {
		OK int `bson:"ok"`
	}
	err = client.Database("admin").RunCommand(ctx, primitive.M{"replSetReconfig": rsConf.Config}).Decode(&replSetReconfig)
	assert.NoError(t, err)

	msclient := tu.TestClient(ctx, tu.MongoDBS1Secondary1Port, t)
	var m bson.M

	cmd := bson.D{{Key: "getDiagnosticData", Value: "1"}}
	res = msclient.Database("admin").RunCommand(ctx, cmd)

	err = res.Decode(&m)
	assert.NoError(t, err)

	m, _ = m["data"].(bson.M)
	lag := replicationLag(m)

	metric := &dto.Metric{}
	err = lag.Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, float64(secondsBehind), *metric.Gauge.Value)

	rsConfOld.Config.Version = rsConf.Config.Version + 1
	err = client.Database("admin").RunCommand(ctx, primitive.M{"replSetReconfig": rsConfOld.Config}).Decode(&replSetReconfig)
	assert.NoError(t, err)
}
