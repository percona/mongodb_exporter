// mongodb_exporter
// Copyright (C) 2024 Percona LLC
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
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:paralleltest
func TestPBMCollector(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	mongoURI := "mongodb://admin:admin@127.0.0.1:17006/?connectTimeoutMS=1000&directConnection=true&serverSelectionTimeoutMS=1000" //nolint:gosec

	// The exporter gives the collector its own client, built from the same URI (see makeRegistry).
	client, err := connect(ctx, &Opts{URI: mongoURI, DirectConnect: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	c := newPbmCollector(ctx, client, mongoURI, 1000, promslog.New(&promslog.Config{}))

	t.Run("pbm configured metric", func(t *testing.T) {
		filter := []string{
			"mongodb_pbm_cluster_backup_configured",
		}
		expected := strings.NewReader(`
		# HELP mongodb_pbm_cluster_backup_configured PBM backups are configured for the cluster
		# TYPE mongodb_pbm_cluster_backup_configured gauge
		mongodb_pbm_cluster_backup_configured 1` + "\n")
		err = testutil.CollectAndCompare(c, expected, filter...)
		assert.NoError(t, err)
	})

	t.Run("pbm agent status metric", func(t *testing.T) {
		filter := []string{
			"mongodb_pbm_agent_status",
		}
		expectedLength := 4 // we expect 4 metrics for each member of the RS (1 primary, 2 secondaries, 1 arbiter).
		count := testutil.CollectAndCount(c, filter...)
		assert.Equal(t, expectedLength, count, "PBM metrics are missing")
	})

	t.Run("pbm agent status values", func(t *testing.T) {
		// mongo-2-1 is a secondary of rs2. The three data members run a PBM agent; the arbiter does not.
		reg := prometheus.NewRegistry()
		require.NoError(t, reg.Register(c))
		families, err := reg.Gather()
		require.NoError(t, err)

		roles := make(map[string]int)
		self := 0
		for _, family := range families {
			if family.GetName() != "mongodb_pbm_agent_status" {
				continue
			}
			for _, m := range family.GetMetric() {
				labels := make(map[string]string)
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				roles[labels["role"]]++
				if labels["self"] == "1" {
					self++
					assert.Equal(t, pbmRoleSecondary, labels["role"])
				}

				want := float64(pbmAgentStatusOK)
				if labels["role"] == pbmRoleArbiter {
					want = pbmAgentStatusError
				}
				assert.InDelta(t, want, m.GetGauge().GetValue(), 0, "%v", labels)
			}
		}

		assert.Equal(t, map[string]int{pbmRolePrimary: 1, pbmRoleSecondary: 2, pbmRoleArbiter: 1}, roles)
		assert.Equal(t, 1, self)
	})

	t.Run("pbm pitr metric", func(t *testing.T) {
		filter := []string{
			"mongodb_pbm_cluster_pitr_backup_enabled",
		}
		expected := strings.NewReader(`
		# HELP mongodb_pbm_cluster_pitr_backup_enabled PBM PITR backups are enabled for the cluster
		# TYPE mongodb_pbm_cluster_pitr_backup_enabled gauge
		mongodb_pbm_cluster_pitr_backup_enabled 0` + "\n")
		err = testutil.CollectAndCompare(c, expected, filter...)
		assert.NoError(t, err)
	})

	t.Run("pbm backup metrics", func(t *testing.T) {
		// One series of each per replica set of each backup. The test cluster setup takes a backup of rs2.
		cur, err := client.Database("admin").Collection(pbmBackupsCollection).Find(ctx, bson.D{})
		require.NoError(t, err)
		var backups []bson.M
		require.NoError(t, cur.All(ctx, &backups))

		want := 0
		for _, backup := range backups {
			replsets, _ := backup["replsets"].(bson.A)
			want += len(replsets)
		}
		require.Positive(t, want, "the test cluster has no PBM backup")

		for _, name := range []string{
			"mongodb_pbm_backup_size_bytes",
			"mongodb_pbm_backup_last_transition_ts",
			"mongodb_pbm_backup_duration_seconds",
		} {
			assert.Equal(t, want, testutil.CollectAndCount(c, name), name)
		}
	})
}

func TestPBMCollectorWithoutPBM(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// The standalone node has no PBM configuration document.
	client := tu.TestClient(ctx, tu.MongoDBStandAlonePort, t)
	c := newPbmCollector(ctx, client, "mongodb://127.0.0.1:"+tu.MongoDBStandAlonePort, 1000, promslog.New(&promslog.Config{}))

	expected := strings.NewReader(`
	# HELP mongodb_pbm_cluster_backup_configured PBM backups are configured for the cluster
	# TYPE mongodb_pbm_cluster_backup_configured gauge
	mongodb_pbm_cluster_backup_configured 0` + "\n")
	err := testutil.CollectAndCompare(c, expected,
		"mongodb_pbm_cluster_backup_configured",
		"mongodb_pbm_cluster_pitr_backup_enabled",
		"mongodb_pbm_agent_status",
		"mongodb_pbm_backup_size_bytes")
	assert.NoError(t, err)
}

// testPBMClusterTime is the cluster time the agent status tests are evaluated at.
var testPBMClusterTime = primitive.Timestamp{T: 1_000_000}

// healthyPBMAgent returns the status document of a healthy agent with a recent heartbeat.
func healthyPBMAgent(state int, stateStr string) *pbmAgent {
	return &pbmAgent{
		State:         state,
		StateStr:      stateStr,
		PBMStatus:     pbmSubsystemStatus{OK: true},
		NodeStatus:    pbmSubsystemStatus{OK: true},
		StorageStatus: pbmSubsystemStatus{OK: true},
		Heartbeat:     primitive.Timestamp{T: testPBMClusterTime.T - 5},
	}
}

func TestPBMNodesWithAgent(t *testing.T) {
	t.Parallel()

	delayed := healthyPBMAgent(memberStateSecondary, "SECONDARY")
	delayed.DelaySecs = 3600
	hidden := healthyPBMAgent(memberStateSecondary, "SECONDARY")
	hidden.Hidden = true
	storageError := healthyPBMAgent(memberStateSecondary, "SECONDARY")
	storageError.StorageStatus = pbmSubsystemStatus{OK: false}
	lost := healthyPBMAgent(memberStateSecondary, "SECONDARY")
	lost.Heartbeat = primitive.Timestamp{T: testPBMClusterTime.T - pbmStaleAgentSecs - 1}
	lostBoundary := healthyPBMAgent(memberStateSecondary, "SECONDARY")
	lostBoundary.Heartbeat = primitive.Timestamp{T: testPBMClusterTime.T - pbmStaleAgentSecs}

	agents := map[string]*pbmAgent{
		"primary:27017":       healthyPBMAgent(memberStatePrimary, "PRIMARY"),
		"secondary:27017":     healthyPBMAgent(memberStateSecondary, "SECONDARY"),
		"arbiter:27017":       healthyPBMAgent(memberStateArbiter, "ARBITER"),
		"delayed:27017":       delayed,
		"hidden:27017":        hidden,
		"recovering:27017":    healthyPBMAgent(3, "RECOVERING"),
		"storage-error:27017": storageError,
		"lost:27017":          lost,
		"lost-boundary:27017": lostBoundary,
	}

	// The member configuration does not matter when the member has an agent.
	members := []pbmMember{
		{Host: "primary:27017"},
		{Host: "secondary:27017"},
		{Host: "arbiter:27017"},
		{Host: "delayed:27017"},
		{Host: "hidden:27017", ArbiterOnly: true},
		{Host: "recovering:27017"},
		{Host: "storage-error:27017"},
		{Host: "lost:27017"},
		{Host: "lost-boundary:27017"},
	}

	assert.Equal(t, []pbmNode{
		{host: "primary:27017", role: pbmRolePrimary, status: pbmAgentStatusOK},
		{host: "secondary:27017", role: pbmRoleSecondary, status: pbmAgentStatusOK},
		{host: "arbiter:27017", role: pbmRoleArbiter, status: pbmAgentStatusOK},
		{host: "delayed:27017", role: pbmRoleDelayed, status: pbmAgentStatusOK},
		{host: "hidden:27017", role: pbmRoleHidden, status: pbmAgentStatusOK},
		{host: "recovering:27017", role: "RECOVERING", status: pbmAgentStatusOK},
		{host: "storage-error:27017", role: pbmRoleSecondary, status: pbmAgentStatusError},
		{host: "lost:27017", role: pbmRoleSecondary, status: pbmAgentStatusLost},
		{host: "lost-boundary:27017", role: pbmRoleSecondary, status: pbmAgentStatusOK},
	}, pbmNodes(members, agents, testPBMClusterTime))
}

func TestPBMNodesWithoutAgent(t *testing.T) {
	t.Parallel()

	// Agents of other replica set members do not count.
	agents := map[string]*pbmAgent{"other:27017": healthyPBMAgent(memberStatePrimary, "PRIMARY")}

	members := []pbmMember{
		{Host: "member:27017"},
		{Host: "arbiter:27017", ArbiterOnly: true},
		{Host: "hidden:27017", Hidden: true},
		{Host: "delayed:27017", SecondaryDelaySecs: 3600},
		{Host: "slave-delay:27017", SlaveDelay: 3600},
		{Host: "hidden-delayed:27017", Hidden: true, SecondaryDelaySecs: 3600},
	}

	assert.Equal(t, []pbmNode{
		{host: "member:27017", role: "", status: pbmAgentStatusError},
		{host: "arbiter:27017", role: pbmRoleArbiter, status: pbmAgentStatusError},
		{host: "hidden:27017", role: pbmRoleHidden, status: pbmAgentStatusError},
		{host: "delayed:27017", role: pbmRoleDelayed, status: pbmAgentStatusError},
		{host: "slave-delay:27017", role: pbmRoleDelayed, status: pbmAgentStatusError},
		{host: "hidden-delayed:27017", role: pbmRoleDelayed, status: pbmAgentStatusError},
	}, pbmNodes(members, agents, testPBMClusterTime))
	assert.Empty(t, pbmNodes(nil, nil, testPBMClusterTime))
}

func TestPBMSourceFor(t *testing.T) {
	t.Parallel()

	primary := &pbmHello{SetName: "rs0", IsMaster: true}
	secondary := &pbmHello{SetName: "rs0"}
	standalone := &pbmHello{IsMaster: true}
	mongos := &pbmHello{IsMaster: true, Msg: typeIsDBGrid}
	// A member started with --replSet that is not initiated yet, or was removed.
	noConfig := &pbmHello{IsReplicaSet: true}

	for _, tc := range []struct {
		role string
		self *pbmHello
		want pbmSource
	}{
		{role: "", self: primary, want: pbmSourceLocal},
		{role: "", self: secondary, want: pbmSourceReplset},
		{role: "", self: standalone, want: pbmSourceLocal},
		{role: "configsvr", self: primary, want: pbmSourceLocal},
		{role: "configsvr", self: secondary, want: pbmSourceReplset},
		{role: string(typeShardServer), self: primary, want: pbmSourceConfigsvr},
		{role: string(typeShardServer), self: secondary, want: pbmSourceConfigsvr},
		{role: string(typeMongos), self: mongos, want: pbmSourceLocal},
	} {
		source, err := pbmSourceFor(tc.role, tc.self)
		require.NoError(t, err)
		assert.Equal(t, tc.want, source, "role %q, %+v", tc.role, *tc.self)
	}

	// Whatever its sharding role, such a member gets no PBM metrics.
	for _, role := range []string{"", "configsvr", string(typeShardServer)} {
		_, err := pbmSourceFor(role, noConfig)
		require.ErrorIs(t, err, errPBMNotReplicaSet, "role %q", role)
	}
}

func TestPBMControlErrorLevel(t *testing.T) {
	t.Parallel()

	_, err := pbmSourceFor("", &pbmHello{IsReplicaSet: true})
	assert.Equal(t, slog.LevelDebug, pbmControlErrorLevel(err))
	assert.Equal(t, slog.LevelWarn, pbmControlErrorLevel(errPBMNoHosts))
}

func TestPBMConnectWithoutHosts(t *testing.T) {
	t.Parallel()

	// The driver would connect to localhost:27017 with no hosts.
	p := &pbmCollector{mongoURI: "mongodb://node1:27017/?directConnection=true"}
	_, err := p.connect(context.Background(), nil)
	require.ErrorIs(t, err, errPBMNoHosts)
}

func TestPBMClientOptions(t *testing.T) {
	t.Parallel()

	// The URI PMM gives the exporter: one node, connected to directly.
	uri := "mongodb://user:p%40ss@node1:27017/?directConnection=true&replicaSet=rs0&tls=true&authSource=admin" //nolint:gosec
	hosts := []string{"cfg1:27019", "cfg2:27019"}

	opts, err := pbmClientOptions(uri, hosts, 5*time.Second)
	require.NoError(t, err)

	assert.Equal(t, hosts, opts.Hosts)
	require.NotNil(t, opts.Direct)
	assert.False(t, *opts.Direct)
	assert.Nil(t, opts.ReplicaSet)
	require.NotNil(t, opts.Auth)
	assert.Equal(t, "user", opts.Auth.Username)
	assert.Equal(t, "p@ss", opts.Auth.Password)
	assert.Equal(t, "admin", opts.Auth.AuthSource)
	assert.NotNil(t, opts.TLSConfig)
	assert.Equal(t, readpref.PrimaryMode, opts.ReadPreference.Mode())
	assert.Equal(t, "majority", opts.ReadConcern.Level)
	require.NoError(t, opts.Validate())

	// Without a connect timeout in the URI, the exporter's applies, as in its own client.
	require.NotNil(t, opts.ConnectTimeout)
	assert.Equal(t, 5*time.Second, *opts.ConnectTimeout)
	require.NotNil(t, opts.ServerSelectionTimeout)
	assert.Equal(t, 5*time.Second, *opts.ServerSelectionTimeout)

	opts, err = pbmClientOptions(uri+"&connectTimeoutMS=1000", hosts, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, opts.ConnectTimeout)
	assert.Equal(t, time.Second, *opts.ConnectTimeout)

	_, err = pbmClientOptions("mongodb://node1:27017,node2:27017/?directConnection=true", hosts, 0)
	assert.Error(t, err)
}

func TestReplsetsFromShardMap(t *testing.T) {
	t.Parallel()

	// Older servers also map every host and shard connection string to its replica set.
	shardMap := map[string]string{
		"config":                     "cfg/cfg1:27019,cfg2:27019",
		"shard01":                    "rs1/rs1a:27018,rs1b:27018",
		"rs1/rs1a:27018,rs1b:27018":  "rs1/rs1a:27018,rs1b:27018",
		"rs1a:27018":                 "rs1/rs1a:27018,rs1b:27018",
		"shard02":                    "rs2/rs2a:27018",
		"standalone-shard-not-valid": "rs3a:27018",
	}

	replsets := replsetsFromShardMap(shardMap)
	sort.Slice(replsets, func(i, j int) bool { return replsets[i].name < replsets[j].name })

	assert.Equal(t, []pbmReplset{
		{name: "cfg", hosts: []string{"cfg1:27019", "cfg2:27019"}},
		{name: "rs1", hosts: []string{"rs1a:27018", "rs1b:27018"}},
		{name: "rs2", hosts: []string{"rs2a:27018"}},
	}, replsets)
}

func TestParseReplsetHosts(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"", "rs0", "rs0/", "/host:27017"} {
		_, ok := parseReplsetHosts(s)
		assert.False(t, ok, s)
	}

	rs, ok := parseReplsetHosts("cfg/cfg1:27019,cfg2:27019")
	require.True(t, ok)
	assert.Equal(t, pbmReplset{name: "cfg", hosts: []string{"cfg1:27019", "cfg2:27019"}}, rs)
}

func TestPBMBackupsMetrics(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_001_000, 0)
	backups := []pbmBackup{
		{
			Type:             "physical",
			OPID:             "op-running",
			Name:             "2023-11-14T22:30:00Z",
			Replsets:         []pbmBackupReplset{{Name: "rs0", Node: "rs0a:27017"}},
			Size:             0,
			StartTS:          1_700_000_000,
			LastTransitionTS: 1_700_000_100,
			Status:           "running",
		},
		{
			Type:             pbmLogicalBackup,
			OPID:             "op-done",
			Name:             "2023-11-14T20:00:00Z",
			Replsets:         []pbmBackupReplset{{Name: "rs0", Node: "rs0b:27017"}},
			Size:             2048,
			StartTS:          1_699_990_000,
			LastTransitionTS: 1_699_990_060,
			Status:           statusDone,
		},
	}

	expected := strings.NewReader(`
	# HELP mongodb_pbm_backup_duration_seconds Duration of PBM backup
	# TYPE mongodb_pbm_backup_duration_seconds gauge
	mongodb_pbm_backup_duration_seconds{host="rs0a:27017",name="2023-11-14T22:30:00Z",opid="op-running",replica_set="rs0",self="1",status="running",type="physical"} 1000
	mongodb_pbm_backup_duration_seconds{host="rs0b:27017",name="2023-11-14T20:00:00Z",opid="op-done",replica_set="rs0",self="0",status="done",type="logical"} 60
	# HELP mongodb_pbm_backup_last_transition_ts Last transition timestamp of PBM backup (seconds since epoch)
	# TYPE mongodb_pbm_backup_last_transition_ts gauge
	mongodb_pbm_backup_last_transition_ts{host="rs0a:27017",name="2023-11-14T22:30:00Z",opid="op-running",replica_set="rs0",self="1",status="running",type="physical"} 1.7000001e+09
	mongodb_pbm_backup_last_transition_ts{host="rs0b:27017",name="2023-11-14T20:00:00Z",opid="op-done",replica_set="rs0",self="0",status="done",type="logical"} 1.69999006e+09
	# HELP mongodb_pbm_backup_size_bytes Size of PBM backup
	# TYPE mongodb_pbm_backup_size_bytes gauge
	mongodb_pbm_backup_size_bytes{host="rs0a:27017",name="2023-11-14T22:30:00Z",opid="op-running",replica_set="rs0",self="1",status="running",type="physical"} 0
	mongodb_pbm_backup_size_bytes{host="rs0b:27017",name="2023-11-14T20:00:00Z",opid="op-done",replica_set="rs0",self="0",status="done",type="logical"} 2048
	`)

	err := testutil.CollectAndCompare(staticCollector(pbmBackupsMetrics(backups, "rs0a:27017", now)), expected)
	assert.NoError(t, err)
}
