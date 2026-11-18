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
	"fmt"
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
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

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

	t.Run("pbm agent version metric", func(t *testing.T) {
		// One series, for the agent of the member the collector is connected to, with the version from the
		// agent's status document.
		hello, err := getPBMHello(ctx, client)
		require.NoError(t, err)
		var agent struct {
			Version string `bson:"v"`
		}
		err = client.Database("admin").Collection(pbmAgentsCollection).
			FindOne(ctx, bson.D{{Key: "n", Value: hello.Me}}).Decode(&agent)
		require.NoError(t, err)
		require.NotEmpty(t, agent.Version)

		reg := prometheus.NewRegistry()
		require.NoError(t, reg.Register(c))
		families, err := reg.Gather()
		require.NoError(t, err)

		var got []map[string]string
		for _, family := range families {
			if family.GetName() != "mongodb_pbm_agent_version_info" {
				continue
			}
			for _, m := range family.GetMetric() {
				labels := make(map[string]string)
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				assert.InDelta(t, 1, m.GetGauge().GetValue(), 0, "%v", labels)
				got = append(got, labels)
			}
		}

		assert.Equal(t, []map[string]string{
			{"host": hello.Me, "replica_set": "rs2", "self": "1", "version": agent.Version},
		}, got)
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

	t.Run("pbm restore metrics", func(t *testing.T) {
		// One status of the most recent restore, and one series of each per recent restore.
		count, err := client.Database("admin").Collection(pbmRestoresCollection).CountDocuments(ctx, bson.D{})
		require.NoError(t, err)
		want := min(count, pbmRestoresListed)

		assert.Equal(t, 1, testutil.CollectAndCount(c, "mongodb_pbm_restore_status"))
		if want == 0 {
			expected := strings.NewReader(`
			# HELP mongodb_pbm_restore_status Status of the most recent PBM restore in admin.pbmRestores: 0 = idle (none recorded), 1 = running, 2 = failed, 3 = completed. A partly done physical restore failed on some nodes, which need an initial sync, and counts as failed. Covers logical restores; physical, incremental and external restores appear only after a resync, and most failed ones never do
			# TYPE mongodb_pbm_restore_status gauge
			mongodb_pbm_restore_status 0` + "\n")
			require.NoError(t, testutil.CollectAndCompare(c, expected, "mongodb_pbm_restore_status"))
		}
		for _, name := range []string{
			"mongodb_pbm_restore_last_transition_ts",
			"mongodb_pbm_restore_duration_seconds",
		} {
			assert.Equal(t, int(want), testutil.CollectAndCount(c, name), name)
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
		"mongodb_pbm_agent_version_info",
		"mongodb_pbm_backup_size_bytes",
		"mongodb_pbm_restore_status",
		"mongodb_pbm_restore_last_transition_ts",
		"mongodb_pbm_restore_duration_seconds")
	assert.NoError(t, err)
}

// testPBMClusterTime is the cluster time the agent status tests are evaluated at.
var testPBMClusterTime = primitive.Timestamp{T: 1_000_000}

// testPBMAgentVersion is the version healthyPBMAgent reports, in the form PBM stores it.
const testPBMAgentVersion = "2.12.0"

// healthyPBMAgent returns the status document of a healthy agent with a recent heartbeat.
func healthyPBMAgent(state int, stateStr string) *pbmAgent {
	return &pbmAgent{
		Version:       testPBMAgentVersion,
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
	// A lost agent keeps the version it last reported.
	lost.Version = "2.11.0"
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

	v := testPBMAgentVersion
	assert.Equal(t, []pbmNode{
		{host: "primary:27017", role: pbmRolePrimary, status: pbmAgentStatusOK, version: v},
		{host: "secondary:27017", role: pbmRoleSecondary, status: pbmAgentStatusOK, version: v},
		{host: "arbiter:27017", role: pbmRoleArbiter, status: pbmAgentStatusOK, version: v},
		{host: "delayed:27017", role: pbmRoleDelayed, status: pbmAgentStatusOK, version: v},
		{host: "hidden:27017", role: pbmRoleHidden, status: pbmAgentStatusOK, version: v},
		{host: "recovering:27017", role: "RECOVERING", status: pbmAgentStatusOK, version: v},
		{host: "storage-error:27017", role: pbmRoleSecondary, status: pbmAgentStatusError, version: v},
		{host: "lost:27017", role: pbmRoleSecondary, status: pbmAgentStatusLost, version: "2.11.0"},
		{host: "lost-boundary:27017", role: pbmRoleSecondary, status: pbmAgentStatusOK, version: v},
	}, pbmNodes(members, agents, testPBMClusterTime))
}

func TestPBMAgentMetrics(t *testing.T) {
	t.Parallel()

	// A member without an agent gets a status but no version.
	status := map[string][]pbmNode{
		"rs0": {
			{host: "rs0a:27017", role: pbmRolePrimary, status: pbmAgentStatusOK, version: "2.12.0"},
			{host: "rs0b:27017", role: pbmRoleSecondary, status: pbmAgentStatusLost, version: "2.11.0"},
			{host: "rs0c:27017", role: pbmRoleArbiter, status: pbmAgentStatusError},
		},
		"cfg": {
			{host: "cfg1:27019", role: pbmRolePrimary, status: pbmAgentStatusOK, version: "2.12.0"},
		},
	}

	for _, tc := range []struct {
		name     string
		me       string
		expected string
	}{
		{
			name: "a member reports its own version",
			me:   "rs0a:27017",
			expected: `
			# HELP mongodb_pbm_agent_status PBM Agent Status
			# TYPE mongodb_pbm_agent_status gauge
			mongodb_pbm_agent_status{host="cfg1:27019",replica_set="cfg",role="P",self="0"} 0
			mongodb_pbm_agent_status{host="rs0a:27017",replica_set="rs0",role="P",self="1"} 0
			mongodb_pbm_agent_status{host="rs0b:27017",replica_set="rs0",role="S",self="0"} 2
			mongodb_pbm_agent_status{host="rs0c:27017",replica_set="rs0",role="A",self="0"} 1
			# HELP mongodb_pbm_agent_version_info The version of the PBM agent
			# TYPE mongodb_pbm_agent_version_info gauge
			mongodb_pbm_agent_version_info{host="rs0a:27017",replica_set="rs0",self="1",version="2.12.0"} 1
			`,
		},
		{
			name: "mongos, which is no member, reports every member's",
			expected: `
			# HELP mongodb_pbm_agent_status PBM Agent Status
			# TYPE mongodb_pbm_agent_status gauge
			mongodb_pbm_agent_status{host="cfg1:27019",replica_set="cfg",role="P",self="0"} 0
			mongodb_pbm_agent_status{host="rs0a:27017",replica_set="rs0",role="P",self="0"} 0
			mongodb_pbm_agent_status{host="rs0b:27017",replica_set="rs0",role="S",self="0"} 2
			mongodb_pbm_agent_status{host="rs0c:27017",replica_set="rs0",role="A",self="0"} 1
			# HELP mongodb_pbm_agent_version_info The version of the PBM agent
			# TYPE mongodb_pbm_agent_version_info gauge
			mongodb_pbm_agent_version_info{host="cfg1:27019",replica_set="cfg",self="0",version="2.12.0"} 1
			mongodb_pbm_agent_version_info{host="rs0a:27017",replica_set="rs0",self="0",version="2.12.0"} 1
			mongodb_pbm_agent_version_info{host="rs0b:27017",replica_set="rs0",self="0",version="2.11.0"} 1
			`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			metrics := pbmAgentStatusMetrics(status, tc.me)
			err := testutil.CollectAndCompare(staticCollector(metrics), strings.NewReader(tc.expected))
			assert.NoError(t, err)
		})
	}
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

// TestPBMCollectorRestores runs the collector on restore metadata shaped like PBM's, through mongos, which has
// PBM's collections but no PBM to change them.
//
//nolint:funlen,paralleltest // it writes a PBM configuration through mongos, which other tests share
func TestPBMCollectorRestores(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := tu.DefaultTestClientMongoS(ctx, t)
	port, err := tu.PortForContainer("mongos")
	require.NoError(t, err)

	majority := options.Collection().SetWriteConcern(writeconcern.Majority())
	config := client.Database("admin").Collection(pbmConfigCollection, majority)
	restores := client.Database("admin").Collection(pbmRestoresCollection, majority)
	ours := bson.D{{Key: "opid", Value: bson.D{{Key: "$regex", Value: "^PMM-15220-"}}}}
	clean := func() {
		_, _ = config.DeleteMany(context.WithoutCancel(ctx), bson.D{})
		_, _ = restores.DeleteMany(context.WithoutCancel(ctx), ours)
	}
	clean()
	t.Cleanup(clean)
	_, err = config.InsertOne(ctx, bson.D{{Key: "pitr", Value: bson.D{{Key: "enabled", Value: false}}}})
	require.NoError(t, err)

	c := newPbmCollector(ctx, client, "mongodb://127.0.0.1:"+port, 1000, promslog.New(&promslog.Config{}))

	t.Run("documents as PBM writes them", func(t *testing.T) {
		_, err := restores.DeleteMany(ctx, ours)
		require.NoError(t, err)

		now := time.Now().Unix()
		nowT := uint32(now) //nolint:gosec // seconds since the epoch fit until 2106
		// Older PBM versions leave the type out of a logical restore's metadata.
		done := bson.D{
			{Key: "opid", Value: "PMM-15220-done"},
			{Key: "name", Value: "2026-11-18T08:00:00.000000001Z"},
			{Key: "backup", Value: "2026-11-18T07:00:00Z"},
			{Key: "start_ts", Value: now - 3600},
			{Key: "last_transition_ts", Value: now - 3000},
			{Key: "status", Value: "done"},
			{Key: "hb", Value: primitive.Timestamp{T: nowT - 3000, I: 1}},
		}
		// Its agents stopped: unfinished, with a heartbeat far behind the cluster time.
		stuck := bson.D{
			{Key: "type", Value: "logical"},
			{Key: "opid", Value: "PMM-15220-stuck"},
			{Key: "name", Value: "2026-11-18T09:00:00.000000001Z"},
			{Key: "backup", Value: "2026-11-18T07:00:00Z"},
			{Key: "start_ts", Value: now - 600},
			{Key: "last_transition_ts", Value: now - 590},
			{Key: "status", Value: "running"},
			{Key: "hb", Value: primitive.Timestamp{T: nowT - 580, I: 1}},
		}
		// done twice, as PBM's agents can record a restore that fails early
		for _, restore := range []bson.D{done, done, stuck} {
			_, err = restores.InsertOne(ctx, restore)
			require.NoError(t, err)
		}

		expected := strings.NewReader(fmt.Sprintf(`
		# HELP mongodb_pbm_restore_duration_seconds Duration of each of the 10 most recent PBM restores
		# TYPE mongodb_pbm_restore_duration_seconds gauge
		mongodb_pbm_restore_duration_seconds{backup="2026-11-18T07:00:00Z",name="2026-11-18T08:00:00.000000001Z",opid="PMM-15220-done",status="done",type="logical"} 600
		mongodb_pbm_restore_duration_seconds{backup="2026-11-18T07:00:00Z",name="2026-11-18T09:00:00.000000001Z",opid="PMM-15220-stuck",status="running",type="logical"} 20
		# HELP mongodb_pbm_restore_last_transition_ts Last transition timestamp of each of the 10 most recent PBM restores (seconds since epoch)
		# TYPE mongodb_pbm_restore_last_transition_ts gauge
		mongodb_pbm_restore_last_transition_ts{backup="2026-11-18T07:00:00Z",name="2026-11-18T08:00:00.000000001Z",opid="PMM-15220-done",status="done",type="logical"} %d
		mongodb_pbm_restore_last_transition_ts{backup="2026-11-18T07:00:00Z",name="2026-11-18T09:00:00.000000001Z",opid="PMM-15220-stuck",status="running",type="logical"} %d
		# HELP mongodb_pbm_restore_status Status of the most recent PBM restore in admin.pbmRestores: 0 = idle (none recorded), 1 = running, 2 = failed, 3 = completed. A partly done physical restore failed on some nodes, which need an initial sync, and counts as failed. Covers logical restores; physical, incremental and external restores appear only after a resync, and most failed ones never do
		# TYPE mongodb_pbm_restore_status gauge
		mongodb_pbm_restore_status 2
		`, now-3000, now-590))
		err = testutil.CollectAndCompare(c, expected,
			"mongodb_pbm_restore_status", "mongodb_pbm_restore_last_transition_ts", "mongodb_pbm_restore_duration_seconds")
		assert.NoError(t, err)
	})

	t.Run("only the most recent restores", func(t *testing.T) {
		_, err := restores.DeleteMany(ctx, ours)
		require.NoError(t, err)

		for i := range pbmRestoresListed + 1 {
			_, err = restores.InsertOne(ctx, bson.D{
				{Key: "opid", Value: fmt.Sprintf("PMM-15220-%d", i)},
				{Key: "name", Value: fmt.Sprintf("restore-%d", i)},
				{Key: "start_ts", Value: int64(1_700_000_000 + i)},
				{Key: "status", Value: "done"},
			})
			require.NoError(t, err)
		}

		assert.Equal(t, pbmRestoresListed, testutil.CollectAndCount(c, "mongodb_pbm_restore_last_transition_ts"))
	})
}

// TestPBMCollectorRestoresWithoutClusterTime covers a node without a cluster time, as a standalone node has
// none: a recorded result still counts, and only an unfinished restore can't be told stale.
//
//nolint:paralleltest // TestPBMCollectorWithoutPBM expects no PBM configuration on the standalone node.
func TestPBMCollectorRestoresWithoutClusterTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// The standalone node has no PBM, so these are the only PBM documents there.
	client := tu.TestClient(ctx, tu.MongoDBStandAlonePort, t)
	config := client.Database("admin").Collection(pbmConfigCollection)
	restores := client.Database("admin").Collection(pbmRestoresCollection)
	clean := func() {
		_, _ = config.DeleteMany(context.WithoutCancel(ctx), bson.D{})
		_, _ = restores.DeleteMany(context.WithoutCancel(ctx), bson.D{})
	}
	clean()
	t.Cleanup(clean)
	_, err := config.InsertOne(ctx, bson.D{{Key: "pitr", Value: bson.D{{Key: "enabled", Value: false}}}})
	require.NoError(t, err)

	c := newPbmCollector(ctx, client, "mongodb://127.0.0.1:"+tu.MongoDBStandAlonePort, 1000, promslog.New(&promslog.Config{}))

	for _, tc := range []struct {
		status pbmBackupStatus
		want   int
	}{
		{status: statusError, want: pbmRestoreFailed},
		{status: "running", want: pbmRestoreRunning},
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			_, err := restores.DeleteMany(ctx, bson.D{})
			require.NoError(t, err)
			// Its heartbeat would make an unfinished restore stale, had the node a cluster time.
			_, err = restores.InsertOne(ctx, bson.D{
				{Key: "opid", Value: "PMM-15220-no-cluster-time"},
				{Key: "name", Value: "2026-11-18T08:00:00.000000001Z"},
				{Key: "start_ts", Value: int64(100)},
				{Key: "status", Value: string(tc.status)},
				{Key: "hb", Value: primitive.Timestamp{T: 100, I: 1}},
			})
			require.NoError(t, err)

			expected := strings.NewReader(fmt.Sprintf(`
			# HELP mongodb_pbm_restore_status Status of the most recent PBM restore in admin.pbmRestores: 0 = idle (none recorded), 1 = running, 2 = failed, 3 = completed. A partly done physical restore failed on some nodes, which need an initial sync, and counts as failed. Covers logical restores; physical, incremental and external restores appear only after a resync, and most failed ones never do
			# TYPE mongodb_pbm_restore_status gauge
			mongodb_pbm_restore_status %d
			`, tc.want))
			err = testutil.CollectAndCompare(c, expected, "mongodb_pbm_restore_status")
			assert.NoError(t, err)
		})
	}
}

//nolint:funlen
func TestPBMRestoresMetrics(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_001_000, 0)
	clusterTime := primitive.Timestamp{T: 1_700_001_000}
	restores := []pbmRestore{
		{
			Type:             pbmLogicalBackup,
			OPID:             "op-running",
			Name:             "2023-11-14T22:30:00.123456789Z",
			Backup:           "2023-11-14T20:00:00Z",
			StartTS:          1_700_000_000,
			LastTransitionTS: 1_700_000_100,
			Status:           "running",
			Heartbeat:        primitive.Timestamp{T: 1_700_000_995},
		},
		{
			Type:             "physical",
			OPID:             "op-done",
			Name:             "2023-11-14T21:00:00.123456789Z",
			Backup:           "2023-11-14T20:00:00Z",
			StartTS:          1_699_995_000,
			LastTransitionTS: 1_699_995_600,
			Status:           statusDone,
		},
		{
			// The same restore again: without a unique index on name, PBM's agents can insert it twice.
			Type:             "physical",
			OPID:             "op-done",
			Name:             "2023-11-14T21:00:00.123456789Z",
			Backup:           "2023-11-14T20:00:00Z",
			StartTS:          1_699_995_000,
			LastTransitionTS: 1_699_995_600,
			Status:           statusDone,
		},
		{
			// Its agents stopped without recording a result.
			Type:             pbmLogicalBackup,
			OPID:             "op-stale",
			Name:             "2023-11-14T20:30:00.123456789Z",
			Backup:           "2023-11-14T20:00:00Z",
			StartTS:          1_699_993_000,
			LastTransitionTS: 1_699_993_001,
			Status:           "starting",
			Heartbeat:        primitive.Timestamp{T: 1_699_993_030},
		},
	}

	expected := strings.NewReader(`
	# HELP mongodb_pbm_restore_duration_seconds Duration of each of the 10 most recent PBM restores
	# TYPE mongodb_pbm_restore_duration_seconds gauge
	mongodb_pbm_restore_duration_seconds{backup="2023-11-14T20:00:00Z",name="2023-11-14T20:30:00.123456789Z",opid="op-stale",status="starting",type="logical"} 30
	mongodb_pbm_restore_duration_seconds{backup="2023-11-14T20:00:00Z",name="2023-11-14T21:00:00.123456789Z",opid="op-done",status="done",type="physical"} 600
	mongodb_pbm_restore_duration_seconds{backup="2023-11-14T20:00:00Z",name="2023-11-14T22:30:00.123456789Z",opid="op-running",status="running",type="logical"} 1000
	# HELP mongodb_pbm_restore_last_transition_ts Last transition timestamp of each of the 10 most recent PBM restores (seconds since epoch)
	# TYPE mongodb_pbm_restore_last_transition_ts gauge
	mongodb_pbm_restore_last_transition_ts{backup="2023-11-14T20:00:00Z",name="2023-11-14T20:30:00.123456789Z",opid="op-stale",status="starting",type="logical"} 1.699993001e+09
	mongodb_pbm_restore_last_transition_ts{backup="2023-11-14T20:00:00Z",name="2023-11-14T21:00:00.123456789Z",opid="op-done",status="done",type="physical"} 1.6999956e+09
	mongodb_pbm_restore_last_transition_ts{backup="2023-11-14T20:00:00Z",name="2023-11-14T22:30:00.123456789Z",opid="op-running",status="running",type="logical"} 1.7000001e+09
	# HELP mongodb_pbm_restore_status Status of the most recent PBM restore in admin.pbmRestores: 0 = idle (none recorded), 1 = running, 2 = failed, 3 = completed. A partly done physical restore failed on some nodes, which need an initial sync, and counts as failed. Covers logical restores; physical, incremental and external restores appear only after a resync, and most failed ones never do
	# TYPE mongodb_pbm_restore_status gauge
	mongodb_pbm_restore_status 1
	`)

	err := testutil.CollectAndCompare(staticCollector(pbmRestoresMetrics(restores, clusterTime, now)), expected)
	assert.NoError(t, err)
}

func TestPBMRestoresMetricsWithoutRestore(t *testing.T) {
	t.Parallel()

	expected := strings.NewReader(`
	# HELP mongodb_pbm_restore_status Status of the most recent PBM restore in admin.pbmRestores: 0 = idle (none recorded), 1 = running, 2 = failed, 3 = completed. A partly done physical restore failed on some nodes, which need an initial sync, and counts as failed. Covers logical restores; physical, incremental and external restores appear only after a resync, and most failed ones never do
	# TYPE mongodb_pbm_restore_status gauge
	mongodb_pbm_restore_status 0
	`)

	err := testutil.CollectAndCompare(staticCollector(pbmRestoresMetrics(nil, testPBMClusterTime, time.Now())), expected)
	assert.NoError(t, err)
}

func TestPBMRestoreStatus(t *testing.T) {
	t.Parallel()

	recent := primitive.Timestamp{T: testPBMClusterTime.T - pbmStaleAgentSecs}
	stale := primitive.Timestamp{T: testPBMClusterTime.T - pbmStaleAgentSecs - 1}
	recentPhysical := primitive.Timestamp{T: testPBMClusterTime.T - pbmStalePhysicalRestoreSecs}
	stalePhysical := primitive.Timestamp{T: testPBMClusterTime.T - pbmStalePhysicalRestoreSecs - 1}

	for _, tc := range []struct {
		restore  pbmRestore
		want     int
		finished bool
	}{
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: "starting", Heartbeat: recent}, want: pbmRestoreRunning},
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: "running", Heartbeat: recent}, want: pbmRestoreRunning},
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: "dumpDone", Heartbeat: recent}, want: pbmRestoreRunning},
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: statusDone, Heartbeat: stale}, want: pbmRestoreCompleted, finished: true},
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: statusError, Heartbeat: recent}, want: pbmRestoreFailed, finished: true},
		// The agents of the restore stopped without recording a result.
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: "running", Heartbeat: stale}, want: pbmRestoreFailed},
		{restore: pbmRestore{Type: pbmLogicalBackup, Status: "running", Heartbeat: recentPhysical}, want: pbmRestoreFailed},
		{restore: pbmRestore{Type: "physical", Status: statusPartlyDone}, want: pbmRestoreFailed, finished: true},
		{restore: pbmRestore{Type: "physical", Status: statusCancelled}, want: pbmRestoreFailed, finished: true},
		{restore: pbmRestore{Type: "physical", Status: "running", Heartbeat: recentPhysical}, want: pbmRestoreRunning},
		{restore: pbmRestore{Type: "physical", Status: "running", Heartbeat: stalePhysical}, want: pbmRestoreFailed},
		{restore: pbmRestore{Type: "incremental", Status: "copyDone", Heartbeat: recentPhysical}, want: pbmRestoreRunning},
		// The storage had no heartbeat of the restore.
		{restore: pbmRestore{Type: "physical", Status: "running"}, want: pbmRestoreFailed},
	} {
		assert.Equal(t, tc.want, tc.restore.status(testPBMClusterTime), "%+v", tc.restore)
		_, finished := tc.restore.result()
		assert.Equal(t, tc.finished, finished, "%+v", tc.restore)
	}

	// Without the cluster time, a recorded result still counts, and an unfinished restore reads as running.
	noClusterTime := primitive.Timestamp{}
	assert.Equal(t, pbmRestoreFailed, (&pbmRestore{Type: pbmLogicalBackup, Status: statusError}).status(noClusterTime))
	assert.Equal(t, pbmRestoreRunning, (&pbmRestore{Type: pbmLogicalBackup, Status: "running", Heartbeat: stale}).status(noClusterTime))
}
