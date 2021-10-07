// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package exporter

import (
	"context"
	"fmt"
	"sync"

	"github.com/percona/percona-toolkit/src/go/mongolib/proto"
	"github.com/percona/percona-toolkit/src/go/mongolib/util"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type mongoDBNodeType string

const (
	labelClusterRole     = "cl_role"
	labelClusterID       = "cl_id"
	labelReplicasetName  = "rs_nm"
	labelReplicasetState = "rs_state"

	typeIsDBGrid                    = "isdbgrid"
	typeMongos      mongoDBNodeType = "mongos"
	typeMongod      mongoDBNodeType = "mongod"
	typeShardServer mongoDBNodeType = "shardsvr"
)

type labelsGetter interface {
	baseLabels() map[string]string
	loadLabels(context.Context) error
}

// This is an object to make it posible to easily reload the labels in case of
// disconnection from the db. Just call loadLabels when required.
type topologyInfo struct {
	// TODO: with https://jira.percona.com/browse/PMM-6435, replace this client pointer
	// by a new connector, able to reconnect if needed. In case of reconnection, we should
	// call loadLabels to refresh the labels because they might have changed
	client *mongo.Client
	rw     sync.RWMutex
	labels map[string]string
}

// ErrCannotGetTopologyLabels Cannot read topology labels.
var ErrCannotGetTopologyLabels = fmt.Errorf("cannot get topology labels")

func newTopologyInfo(ctx context.Context, client *mongo.Client) (*topologyInfo, error) {
	ti := &topologyInfo{
		client: client,
		labels: make(map[string]string),
	}

	err := ti.loadLabels(ctx)
	if err != nil {
		return nil, err
	}

	return ti, nil
}

// baseLabels returns a copy of the topology labels because in some collectors like
// collstats collector, we must use these base labels and add the namespace or other labels.
func (t *topologyInfo) baseLabels() map[string]string {
	c := map[string]string{}

	t.rw.RLock()
	for k, v := range t.labels {
		c[k] = v
	}
	t.rw.RUnlock()

	return c
}

// TopologyLabels reads several values from MongoDB instance like replicaset name, and other
// topology information and returns a map of labels used to better identify the current monitored instance.
func (t *topologyInfo) loadLabels(ctx context.Context) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.labels = make(map[string]string)

	nodeType, err := getNodeType(ctx, t.client)
	if err != nil {
		return errors.Wrap(err, "cannot get node type for topology info")
	}

	t.labels[labelClusterRole] = string(nodeType)

	// Standalone instances or mongos instances won't have a replicaset name
	if rs, err := util.ReplicasetConfig(ctx, t.client); err == nil {
		t.labels[labelReplicasetName] = rs.Config.ID
	}

	cid, err := util.ClusterID(ctx, t.client)
	if err != nil {
		return errors.Wrapf(ErrCannotGetTopologyLabels, "error getting cluster ID: %s", err)
	}
	t.labels[labelClusterID] = cid

	// Standalone instances or mongos instances won't have a replicaset state
	state, err := util.MyState(ctx, t.client)
	if err == nil {
		t.labels[labelReplicasetState] = fmt.Sprintf("%d", state)
	}

	return nil
}

func getNodeType(ctx context.Context, client *mongo.Client) (mongoDBNodeType, error) {
	md := proto.MasterDoc{}
	if err := client.Database("admin").RunCommand(ctx, primitive.M{"isMaster": 1}).Decode(&md); err != nil {
		return "", err
	}

	if md.SetName != nil || md.Hosts != nil {
		return typeShardServer, nil
	} else if md.Msg == typeIsDBGrid {
		// isdbgrid is always the msg value when calling isMaster on a mongos
		// see http://docs.mongodb.org/manual/core/sharded-cluster-query-router/
		return typeMongos, nil
	}

	return typeMongod, nil
}
