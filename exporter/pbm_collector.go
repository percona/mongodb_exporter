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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"golang.org/x/sync/errgroup"

	"github.com/percona/mongodb_exporter/exporter/dsn_fix"
	"github.com/percona/mongodb_exporter/internal/proto"
	"github.com/percona/mongodb_exporter/internal/redact"
)

// pbm collector collects metrics from PBM (Percona Backup for MongoDb).
//
// It reads PBM's control collections with the MongoDB driver instead of using the PBM SDK. The SDK
// imports every PBM storage backend (S3, Azure, GCS, MinIO, ...), which nearly doubled the exporter
// binary and the memory it has resident after a start.
type pbmCollector struct {
	ctx      context.Context
	mongoURI string
	base     *baseCollector

	// connectTimeout is the exporter's --mongodb.connect-timeout-ms.
	connectTimeout time.Duration
}

type pbmAgentStatus string

const (
	pbmAgentStatusOK    = iota
	pbmAgentStatusError = iota
	pbmAgentStatusLost  = iota

	statusDown      pbmAgentStatus = "down"
	statusDone      pbmAgentStatus = "done"
	statusCancelled pbmAgentStatus = "canceled"
	statusError     pbmAgentStatus = "error"
)

// PBM keeps its state in these admin collections of the replica set or, in a sharded cluster, of the
// config server replica set.
const (
	pbmConfigCollection  = "pbmConfig"
	pbmAgentsCollection  = "pbmAgents"
	pbmBackupsCollection = "pbmBackups"

	// pbmStaleAgentSecs is PBM's defs.StaleFrameSec: an agent whose last heartbeat is further behind
	// the cluster time is lost.
	pbmStaleAgentSecs = 30

	// pbmLogicalBackup is the type PBM assumes for backup metadata that has none.
	pbmLogicalBackup = "logical"

	// pbmDisconnectTimeout bounds closing a short-lived client.
	pbmDisconnectTimeout = 5 * time.Second
)

// Values of the role label, the same as PBM's topo.NodeRole.
const (
	pbmRolePrimary   = "P"
	pbmRoleSecondary = "S"
	pbmRoleArbiter   = "A"
	pbmRoleHidden    = "H"
	pbmRoleDelayed   = "D"
)

// Replica set member states, as PBM agents report them.
const (
	memberStatePrimary   = 1
	memberStateSecondary = 2
	memberStateArbiter   = 7
)

var (
	errPBMNoClusterTime    = errors.New("no cluster time in the hello response")
	errPBMNotReplicaSet    = errors.New("not a replica set member")
	errPBMConfigsvrAddress = errors.New("unexpected config server address")
	errPBMNoHosts          = errors.New("no hosts to connect to")
)

// pbmConfig is the part of PBM's main configuration document that the collector reads.
type pbmConfig struct {
	PITR struct {
		Enabled bool `bson:"enabled"`
	} `bson:"pitr"`
}

// pbmSubsystemStatus is one of the health checks a PBM agent reports.
type pbmSubsystemStatus struct {
	OK bool `bson:"ok"`
}

// pbmAgent is the part of a PBM agent status document that the collector reads.
type pbmAgent struct {
	Node          string              `bson:"n"`
	RS            string              `bson:"rs"`
	State         int                 `bson:"s"`
	StateStr      string              `bson:"str"`
	Hidden        bool                `bson:"hdn"`
	DelaySecs     int32               `bson:"delay"`
	PBMStatus     pbmSubsystemStatus  `bson:"pbms"`
	NodeStatus    pbmSubsystemStatus  `bson:"nodes"`
	StorageStatus pbmSubsystemStatus  `bson:"stors"`
	Heartbeat     primitive.Timestamp `bson:"hb"`
}

// pbmBackupType and pbmBackupStatus are PBM's defs.BackupType and defs.Status.
type (
	pbmBackupType   string
	pbmBackupStatus string
)

// pbmBackup is the part of PBM backup metadata that the collector reads.
type pbmBackup struct {
	Type             pbmBackupType      `bson:"type"`
	OPID             string             `bson:"opid"`
	Name             string             `bson:"name"`
	Replsets         []pbmBackupReplset `bson:"replsets"`
	Size             int64              `bson:"size"`
	StartTS          int64              `bson:"start_ts"`
	LastTransitionTS int64              `bson:"last_transition_ts"`
	Status           pbmBackupStatus    `bson:"status"`
}

// pbmBackupProjection keeps the fields of pbmBackup out of PBM backup metadata, which also holds file
// lists and storage credentials.
//
//nolint:goconst // PBM's field names
var pbmBackupProjection = bson.D{
	{Key: "type", Value: 1},
	{Key: "opid", Value: 1},
	{Key: "name", Value: 1},
	{Key: "replsets.name", Value: 1},
	{Key: "replsets.node", Value: 1},
	{Key: "size", Value: 1},
	{Key: "start_ts", Value: 1},
	{Key: "last_transition_ts", Value: 1},
	{Key: "status", Value: 1},
}

type pbmBackupReplset struct {
	Name string `bson:"name"`
	Node string `bson:"node"`
}

// pbmMember is the part of a replica set configuration member that the collector reads.
type pbmMember struct {
	Host               string `bson:"host"`
	ArbiterOnly        bool   `bson:"arbiterOnly"`
	Hidden             bool   `bson:"hidden"`
	SlaveDelay         int64  `bson:"slaveDelay"`
	SecondaryDelaySecs int64  `bson:"secondaryDelaySecs"`
}

// pbmReplset is a replica set of the cluster and the seed list to connect to it.
type pbmReplset struct {
	name  string
	hosts []string
}

// pbmNode is the PBM agent status of one replica set member.
type pbmNode struct {
	host   string
	role   string
	status int
}

// pbmHello is the part of a hello response that the collector uses.
type pbmHello struct {
	Me       string   `bson:"me"`
	SetName  string   `bson:"setName"`
	Hosts    []string `bson:"hosts"`
	IsMaster bool     `bson:"ismaster"`
	Msg      string   `bson:"msg"`
	// IsReplicaSet is set by a member started with --replSet that has no valid replica set
	// configuration (not initiated yet, or removed), which answers without a set name.
	IsReplicaSet bool `bson:"isreplicaset"`
	ClusterTime  *struct {
		ClusterTime primitive.Timestamp `bson:"clusterTime"`
	} `bson:"$clusterTime"`
}

// pbmSource is where the collector reads PBM's collections from.
type pbmSource int

const (
	pbmSourceLocal     pbmSource = iota // the exporter's own client
	pbmSourceReplset                    // the primary of the replica set the exporter is connected to
	pbmSourceConfigsvr                  // the primary of the config server replica set
)

func createPBMMetric(name, help string, value float64, labels map[string]string) prometheus.Metric { //nolint:ireturn
	const prefix = "mongodb_pbm_"
	d := prometheus.NewDesc(prefix+name, help, nil, labels)
	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, value)
}

// newPbmCollector returns a PBM collector. connectTimeoutMS is the exporter's --mongodb.connect-timeout-ms,
// used like the exporter uses it when the URI sets no connect timeout.
func newPbmCollector(ctx context.Context, client *mongo.Client, mongoURI string, connectTimeoutMS int, logger *slog.Logger) *pbmCollector {
	return &pbmCollector{
		ctx:            ctx,
		mongoURI:       mongoURI,
		connectTimeout: time.Duration(connectTimeoutMS) * time.Millisecond,
		base:           newBaseCollector(client, logger.With("collector", "pbm")),
	}
}

func (p *pbmCollector) Describe(ch chan<- *prometheus.Desc) {
	p.base.Describe(p.ctx, ch, p.collect)
}

func (p *pbmCollector) Collect(ch chan<- prometheus.Metric) {
	p.base.Collect(ch)
}

func (p *pbmCollector) collect(ch chan<- prometheus.Metric) {
	defer measureCollectTime(ch, "mongodb", "pbm")()

	var metrics []prometheus.Metric
	logger := p.base.logger

	pbmEnabledMetric := 0
	currentNode, err := getPBMHello(p.ctx, p.base.client)
	if err != nil {
		logger.Error("failed to get current node info", "error", err.Error())

		return
	}

	role, err := pbmClusterRole(p.ctx, p.base.client, currentNode)
	if err != nil {
		logger.Warn("failed to get the cluster role", "error", err.Error())

		return
	}

	control, closeControl, err := p.controlClient(p.ctx, role, currentNode)
	if err != nil {
		// The error can carry the URI the collector connected with.
		logger.Warn("failed to connect to the PBM control collections", "error", redact.Error(err))

		return
	}
	defer closeControl()

	pbmConfig, err := getPBMConfig(p.ctx, control)
	if err != nil {
		logger.Info("failed to get PBM configuration", "error", err.Error())
	}

	if pbmConfig != nil {
		pbmEnabledMetric = 1

		pitrEnabledMetric := 0
		if pbmConfig.PITR.Enabled {
			pitrEnabledMetric = 1
		}

		metrics = append(metrics, createPBMMetric("cluster_pitr_backup_enabled",
			"PBM PITR backups are enabled for the cluster",
			float64(pitrEnabledMetric), nil))

		metrics = append(metrics, p.pbmBackupsMetrics(p.ctx, control, logger, currentNode)...)
		metrics = append(metrics, p.pbmAgentMetrics(p.ctx, control, role, logger, currentNode)...)
	}

	metrics = append(metrics, createPBMMetric("cluster_backup_configured",
		"PBM backups are configured for the cluster",
		float64(pbmEnabledMetric), nil))

	for _, metric := range metrics {
		ch <- metric
	}
}

// getPBMHello returns the hello response of the node a client is connected to (or, through a client to
// a replica set, of its primary).
func getPBMHello(ctx context.Context, client *mongo.Client) (*pbmHello, error) {
	var hello pbmHello
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "isMaster", Value: 1}}).Decode(&hello)
	if err != nil {
		return nil, fmt.Errorf("isMaster: %w", err)
	}

	return &hello, nil
}

// pbmClusterRole returns the sharding role of the node: mongos, configsvr, shardsvr, or "" when it is
// not part of a sharded cluster. Like PBM, it takes the role of a mongod from its command line options
// and fails when it cannot read them.
func pbmClusterRole(ctx context.Context, client *mongo.Client, self *pbmHello) (string, error) {
	if self.Msg == typeIsDBGrid {
		return string(typeMongos), nil
	}

	var opts struct {
		Parsed struct {
			Sharding struct {
				ClusterRole string `bson:"clusterRole"`
			} `bson:"sharding"`
		} `bson:"parsed"`
	}
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "getCmdLineOpts", Value: 1}}).Decode(&opts) //nolint:goconst
	if err != nil {
		return "", fmt.Errorf("getCmdLineOpts: %w", err)
	}

	return opts.Parsed.Sharding.ClusterRole, nil
}

// pbmSourceFor returns where PBM's collections are read from on a node with the given sharding role.
// PBM keeps them on the replica set itself or, in a sharded cluster, on the config server replica set,
// and reads them from the primary. A node the exporter is directly connected to can be a lagging,
// delayed or recovering secondary, so the primary is reached through a separate client.
func pbmSourceFor(role string, self *pbmHello) (pbmSource, error) {
	switch {
	case self.SetName == "" && self.IsReplicaSet:
		// PBM cannot reach a primary through such a member either, so it gets no PBM metrics.
		return 0, fmt.Errorf("%w: it has no valid replica set configuration", errPBMNotReplicaSet)
	case role == string(typeShardServer):
		return pbmSourceConfigsvr, nil
	case role == string(typeMongos), self.IsMaster, self.SetName == "":
		return pbmSourceLocal, nil
	}

	return pbmSourceReplset, nil
}

// controlClient returns a client for the admin database that holds PBM's collections, and a function
// that releases it: the exporter's own client, or a short-lived one to the primary that holds them.
func (p *pbmCollector) controlClient(ctx context.Context, role string, self *pbmHello) (*mongo.Client, func(), error) {
	noop := func() {}

	source, err := pbmSourceFor(role, self)
	if err != nil {
		return nil, noop, err
	}

	var rs pbmReplset
	switch source {
	case pbmSourceLocal:
		return p.base.client, noop, nil
	case pbmSourceReplset:
		rs = pbmReplset{name: self.SetName, hosts: self.Hosts}
	case pbmSourceConfigsvr:
		rs, err = p.configsvrReplset(ctx, self)
		if err != nil {
			return nil, noop, err
		}
	}

	client, err := p.connect(ctx, rs.hosts)
	if err != nil {
		return nil, noop, fmt.Errorf("cannot connect to replica set %s: %w", rs.name, err)
	}

	return client, func() { p.disconnect(ctx, client) }, nil
}

// configsvrReplset returns the config server replica set of the shard the exporter is connected to,
// from the shard identity document. PBM reads that document from the shard's primary; the collector
// reads it from the node it is connected to and goes to the primary only when that node cannot be read,
// a recovering member for example.
func (p *pbmCollector) configsvrReplset(ctx context.Context, self *pbmHello) (pbmReplset, error) {
	identity, err := getShardIdentity(ctx, p.base.client)
	if err != nil && !self.IsMaster {
		identity, err = p.shardIdentityFromPrimary(ctx, self)
	}
	if err != nil {
		return pbmReplset{}, fmt.Errorf("cannot get the config server address: %w", err)
	}

	rs, ok := parseReplsetHosts(identity.ConfigsvrConnectionString)
	if !ok {
		return pbmReplset{}, fmt.Errorf("%w: %q", errPBMConfigsvrAddress, identity.ConfigsvrConnectionString)
	}

	return rs, nil
}

func getShardIdentity(ctx context.Context, client *mongo.Client) (*proto.ShardIdentity, error) {
	var identity proto.ShardIdentity
	err := client.Database("admin").Collection("system.version").
		FindOne(ctx, bson.M{"_id": "shardIdentity"}).Decode(&identity) //nolint:goconst
	if err != nil {
		return nil, fmt.Errorf("shardIdentity: %w", err)
	}

	return &identity, nil
}

func (p *pbmCollector) shardIdentityFromPrimary(ctx context.Context, self *pbmHello) (*proto.ShardIdentity, error) {
	shard, err := p.connect(ctx, self.Hosts)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to replica set %s: %w", self.SetName, err)
	}
	defer p.disconnect(ctx, shard)

	return getShardIdentity(ctx, shard)
}

// pbmClientOptions returns the options of a client to a replica set by its seed list: the credentials
// and TLS settings of the exporter's URI, with the primary read preference and the majority read
// concern PBM uses. connectTimeout applies when the URI sets no connect timeout, as in the exporter's
// own client.
func pbmClientOptions(uri string, hosts []string, connectTimeout time.Duration) (*options.ClientOptions, error) {
	opts, err := dsn_fix.ClientOptionsForDSN(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid dsn: %w", err)
	}

	// The exporter's URI points at one node, often with directConnection=true; the replica set is
	// discovered from the seed list instead.
	opts.SetHosts(hosts).SetDirect(false).SetAppName("mongodb_exporter").
		SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Majority())
	opts.ReplicaSet = nil
	if opts.ConnectTimeout == nil && connectTimeout > 0 {
		opts.SetConnectTimeout(connectTimeout).SetServerSelectionTimeout(connectTimeout)
	}

	return opts, nil
}

// connect opens a client to a replica set by its seed list and checks that it reaches the primary, so
// an unreachable replica set fails here rather than as a missing PBM configuration.
func (p *pbmCollector) connect(ctx context.Context, hosts []string) (*mongo.Client, error) {
	// With no hosts, the driver would connect to localhost:27017.
	if len(hosts) == 0 {
		return nil, errPBMNoHosts
	}

	opts, err := pbmClientOptions(p.mongoURI, hosts, p.connectTimeout)
	if err != nil {
		return nil, err
	}

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("cannot connect: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		p.disconnect(ctx, client)

		return nil, fmt.Errorf("ping: %w", err)
	}

	return client, nil
}

// disconnect closes a short-lived client. It also runs after the scrape context is cancelled (a failed
// replica set read, a scrape deadline), so it doesn't inherit the cancellation: the driver would then
// skip ending the client's server sessions, as PBM avoided by disconnecting with context.Background().
func (p *pbmCollector) disconnect(ctx context.Context, client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pbmDisconnectTimeout)
	defer cancel()

	err := client.Disconnect(ctx)
	if err != nil {
		p.base.logger.Error("failed to close PBM client", "error", err)
	}
}

// pbmAdminDB returns the admin database with the primary read preference and the majority read concern
// PBM reads its collections with, whatever the read preference of the exporter's URI.
func pbmAdminDB(client *mongo.Client) *mongo.Database {
	return client.Database("admin", options.Database().
		SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Majority()))
}

func getPBMConfig(ctx context.Context, client *mongo.Client) (*pbmConfig, error) {
	var cfg pbmConfig
	err := pbmAdminDB(client).Collection(pbmConfigCollection).
		FindOne(ctx, bson.D{{Key: "profile", Value: nil}},
			options.FindOne().SetProjection(bson.D{{Key: "pitr.enabled", Value: 1}})).Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	return &cfg, nil
}

func (p *pbmCollector) pbmAgentMetrics(ctx context.Context, control *mongo.Client, role string, l *slog.Logger, currentNode *pbmHello) []prometheus.Metric {
	clusterStatus, err := p.clusterStatus(ctx, control, role, currentNode)
	if err != nil {
		// The error can carry the URI the collector connected with.
		l.Error("failed to get cluster status", "error", redact.Error(err))
		return nil
	}

	metrics := make([]prometheus.Metric, 0, len(clusterStatus))
	for replsetName, nodes := range clusterStatus {
		for _, node := range nodes {
			labels := map[string]string{
				"host":        node.host,
				"replica_set": replsetName,
				"role":        node.role,
				"self":        "0",
			}
			if node.host == currentNode.Me {
				labels["self"] = "1"
			}
			metrics = append(metrics, createPBMMetric("agent_status",
				"PBM Agent Status",
				float64(node.status),
				labels,
			),
			)
		}
	}

	return metrics
}

// clusterStatus returns the PBM agent status of every member of every replica set in the cluster,
// the way PBM's status command (cli.ClusterStatus) does.
func (p *pbmCollector) clusterStatus(ctx context.Context, control *mongo.Client, role string, currentNode *pbmHello) (map[string][]pbmNode, error) {
	// The cluster time is read from the node the collections are read from.
	hello := currentNode
	if control != p.base.client {
		var err error
		hello, err = getPBMHello(ctx, control)
		if err != nil {
			return nil, fmt.Errorf("read cluster time: %w", err)
		}
	}
	if hello.ClusterTime == nil {
		return nil, errPBMNoClusterTime
	}

	agents, err := listPBMAgents(ctx, control)
	if err != nil {
		return nil, fmt.Errorf("get agent statuses: %w", err)
	}

	replsets, err := clusterReplsets(ctx, control, role, currentNode)
	if err != nil {
		return nil, fmt.Errorf("get cluster members: %w", err)
	}

	// Replica sets whose configuration can be read from their primary without a new connection.
	clients := make(map[string]*mongo.Client)
	if currentNode.IsMaster && currentNode.SetName != "" {
		clients[currentNode.SetName] = p.base.client
	}
	if hello.SetName != "" {
		clients[hello.SetName] = control
	}

	// Like PBM, read the replica set configurations in parallel.
	var mu sync.Mutex
	status := make(map[string][]pbmNode, len(replsets))
	group, groupCtx := errgroup.WithContext(ctx)
	for _, rs := range replsets {
		group.Go(func() error {
			members, err := p.replsetMembers(groupCtx, rs, clients[rs.name])
			if err != nil {
				return fmt.Errorf("get replset status for `%s`: %w", rs.name, err)
			}
			nodes := pbmNodes(members, agents[rs.name], hello.ClusterTime.ClusterTime)

			mu.Lock()
			defer mu.Unlock()
			status[rs.name] = nodes

			return nil
		})
	}
	err = group.Wait()
	if err != nil {
		return nil, err //nolint:wrapcheck // the goroutines wrap their errors
	}

	return status, nil
}

// listPBMAgents returns the PBM agent status documents by replica set name and node.
func listPBMAgents(ctx context.Context, client *mongo.Client) (map[string]map[string]*pbmAgent, error) {
	cur, err := pbmAdminDB(client).Collection(pbmAgentsCollection).Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	var list []pbmAgent
	err = cur.All(ctx, &list)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	agents := make(map[string]map[string]*pbmAgent)
	for i := range list {
		agent := &list[i]
		if agents[agent.RS] == nil {
			agents[agent.RS] = make(map[string]*pbmAgent)
		}
		agents[agent.RS][agent.Node] = agent
	}

	return agents, nil
}

// clusterReplsets returns the replica sets of the cluster: this one when it is not sharded, otherwise
// the shards and the config server replica set.
func clusterReplsets(ctx context.Context, control *mongo.Client, role string, currentNode *pbmHello) ([]pbmReplset, error) {
	if role == "" {
		if currentNode.SetName == "" {
			return nil, errPBMNotReplicaSet
		}

		return []pbmReplset{{name: currentNode.SetName, hosts: currentNode.Hosts}}, nil
	}

	var shardMap struct {
		Map map[string]string `bson:"map"`
	}
	err := control.Database("admin").RunCommand(ctx, bson.D{{Key: "getShardMap", Value: 1}}).Decode(&shardMap)
	if err != nil {
		return nil, fmt.Errorf("getShardMap: %w", err)
	}

	return replsetsFromShardMap(shardMap.Map), nil
}

// replsetsFromShardMap turns the getShardMap map, whose values look like `rs0/host1:27017,host2:27017`
// (the key of the config server replica set is `config`), into one entry per replica set.
func replsetsFromShardMap(shardMap map[string]string) []pbmReplset {
	seen := make(map[string]bool, len(shardMap))
	replsets := make([]pbmReplset, 0, len(shardMap))
	for _, value := range shardMap {
		rs, ok := parseReplsetHosts(value)
		if !ok || seen[rs.name] {
			continue
		}
		seen[rs.name] = true
		replsets = append(replsets, rs)
	}

	return replsets
}

// parseReplsetHosts parses a replica set connection string like `rs0/host1:27017,host2:27017`.
func parseReplsetHosts(s string) (pbmReplset, bool) {
	name, hosts, ok := strings.Cut(s, "/")
	if !ok || name == "" || hosts == "" {
		return pbmReplset{}, false
	}

	return pbmReplset{name: name, hosts: strings.Split(hosts, ",")}, true
}

// replsetMembers returns the members of a replica set from its configuration. It reads the
// configuration with client when one is given, otherwise with a short-lived client to the set.
func (p *pbmCollector) replsetMembers(ctx context.Context, rs pbmReplset, client *mongo.Client) ([]pbmMember, error) {
	if client == nil {
		c, err := p.connect(ctx, rs.hosts)
		if err != nil {
			return nil, err
		}
		defer p.disconnect(ctx, c)
		client = c
	}

	var res struct {
		Config struct {
			Members []pbmMember `bson:"members"`
		} `bson:"config"`
	}
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetConfig", Value: 1}}).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("replSetGetConfig: %w", err)
	}

	return res.Config.Members, nil
}

// pbmNodes returns the PBM agent status of each replica set member, with the same role and status
// rules as PBM's cli.ClusterStatus. agents holds the agent status documents of this replica set by
// node.
func pbmNodes(members []pbmMember, agents map[string]*pbmAgent, clusterTime primitive.Timestamp) []pbmNode {
	nodes := make([]pbmNode, len(members))
	for i, member := range members {
		node := &nodes[i]
		node.host = member.Host
		node.status = pbmAgentStatusError

		agent := agents[member.Host]
		if agent == nil {
			node.role = member.role()

			continue
		}

		node.role = agent.role()
		switch {
		case agent.Heartbeat.T+pbmStaleAgentSecs < clusterTime.T:
			node.status = pbmAgentStatusLost
		case agent.PBMStatus.OK && agent.NodeStatus.OK && agent.StorageStatus.OK:
			node.status = pbmAgentStatusOK
		}
	}

	return nodes
}

// role is the role of a member without a PBM agent, from its replica set configuration.
func (m pbmMember) role() string {
	switch {
	case m.ArbiterOnly:
		return pbmRoleArbiter
	case m.SlaveDelay != 0 || m.SecondaryDelaySecs != 0:
		return pbmRoleDelayed
	case m.Hidden:
		return pbmRoleHidden
	}

	return ""
}

// role is the role of a member as its PBM agent reports it.
func (a *pbmAgent) role() string {
	switch a.State {
	case memberStatePrimary:
		return pbmRolePrimary
	case memberStateArbiter:
		return pbmRoleArbiter
	case memberStateSecondary:
		switch {
		case a.DelaySecs != 0:
			return pbmRoleDelayed
		case a.Hidden:
			return pbmRoleHidden
		}

		return pbmRoleSecondary
	}

	// Unexpected state: show it as it is.
	return a.StateStr
}

func listPBMBackups(ctx context.Context, client *mongo.Client) ([]pbmBackup, error) {
	cur, err := pbmAdminDB(client).Collection(pbmBackupsCollection).
		Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "start_ts", Value: -1}}).SetProjection(pbmBackupProjection))
	if err != nil {
		return nil, fmt.Errorf("query mongo: %w", err)
	}

	var backups []pbmBackup
	err = cur.All(ctx, &backups)
	if err != nil {
		return nil, fmt.Errorf("message decode: %w", err)
	}

	for i := range backups {
		if backups[i].Type == "" {
			backups[i].Type = pbmLogicalBackup
		}
	}

	return backups, nil
}

func (p *pbmCollector) pbmBackupsMetrics(ctx context.Context, control *mongo.Client, l *slog.Logger, currentNode *pbmHello) []prometheus.Metric {
	backupsList, err := listPBMBackups(ctx, control)
	if err != nil {
		l.Error("failed to get PBM backup list", "error", err.Error())
		return nil
	}

	return pbmBackupsMetrics(backupsList, currentNode.Me, time.Now())
}

func pbmBackupsMetrics(backupsList []pbmBackup, me string, now time.Time) []prometheus.Metric {
	metrics := make([]prometheus.Metric, 0, len(backupsList))

	for _, backup := range backupsList {
		// Iterate through replsets in the backup metadata
		for _, replset := range backup.Replsets {
			// Determine if this is the current node
			self := "0"
			if replset.Node == me {
				self = "1"
			}

			labels := map[string]string{
				"opid":        backup.OPID,
				"status":      string(backup.Status),
				"name":        backup.Name,
				"host":        replset.Node,
				"replica_set": replset.Name,
				"self":        self,
				"type":        string(backup.Type),
			}

			metrics = append(metrics, createPBMMetric("backup_size_bytes",
				"Size of PBM backup",
				float64(backup.Size), labels),
			)

			// Add backup_last_transition_ts metric
			metrics = append(metrics, createPBMMetric("backup_last_transition_ts",
				"Last transition timestamp of PBM backup (seconds since epoch)",
				float64(backup.LastTransitionTS), labels),
			)

			var endTime int64
			switch pbmAgentStatus(backup.Status) {
			case statusDone, statusCancelled, statusError, statusDown:
				endTime = backup.LastTransitionTS
			default:
				endTime = now.Unix()
			}

			duration := time.Unix(endTime-backup.StartTS, 0).Unix()
			metrics = append(metrics, createPBMMetric("backup_duration_seconds",
				"Duration of PBM backup",
				float64(duration), labels),
			)
		}
	}
	return metrics
}

var _ prometheus.Collector = (*pbmCollector)(nil)
