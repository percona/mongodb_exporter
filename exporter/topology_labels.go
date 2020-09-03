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

	"github.com/Percona-Lab/mdbutils"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	labelClusterRole     = "cl_role"
	labelClusterID       = "cl_id"
	labelReplicasetName  = "rs_nm"
	labelReplicasetState = "rs_state"
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
	lock   *sync.Mutex
	labels map[string]string
}

// ErrCannotGetTopologyLabels Cannot read topology labels.
var ErrCannotGetTopologyLabels = fmt.Errorf("cannot get topology labels")

func newTopologyInfo(ctx context.Context, client *mongo.Client) (labelsGetter, error) {
	ti := &topologyInfo{
		client: client,
		lock:   &sync.Mutex{},
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
func (t topologyInfo) baseLabels() map[string]string {
	c := map[string]string{}

	for k, v := range t.labels {
		c[k] = v
	}

	return c
}

// TopologyLabels reads several values from MongoDB instance like replicaset name, and other
// topology information and returns a map of labels used to better identify the current monitored instance.
func (t *topologyInfo) loadLabels(ctx context.Context) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.labels = make(map[string]string)

	hi, err := mdbutils.GetHostInfo(ctx, t.client)
	if err != nil {
		return errors.Wrapf(ErrCannotGetTopologyLabels, "error getting host info: %s", err)
	}
	t.labels[labelClusterRole] = hi.NodeType

	// Standalone instances or mongos instances won't have a replicaset name
	if rs, err := mdbutils.ReplicasetConfig(ctx, t.client); err == nil {
		t.labels[labelReplicasetName] = rs.Config.ID
	}

	cid, err := mdbutils.ClusterID(ctx, t.client)
	if err != nil {
		return errors.Wrapf(ErrCannotGetTopologyLabels, "error getting cluster ID: %s", err)
	}
	t.labels[labelClusterID] = cid

	state, err := mdbutils.MyState(ctx, t.client)
	if err != nil {
		return errors.Wrapf(ErrCannotGetTopologyLabels, "error getting replicaset state: %s", err)
	}
	t.labels[labelReplicasetState] = fmt.Sprintf("%d", state)

	return nil
}
