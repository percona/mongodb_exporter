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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestTopologyLabels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	ti, err := newTopologyInfo(ctx, client)
	require.NoError(t, err)
	bl := ti.baseLabels()

	assert.Equal(t, "rs1", bl[labelReplicasetName])
	assert.Equal(t, "1", bl[labelReplicasetState])
	assert.Equal(t, "shardsvr", bl[labelClusterRole])
	assert.NotEmpty(t, bl[labelClusterID]) // this is variable inside a container
}

func TestGetClusterRole(t *testing.T) {
	tests := []struct {
		containerName string
		want          string
	}{
		{
			containerName: "mongos",
			want:          string(typeMongos),
		},
		{
			containerName: "mongo-1-1",
			want:          string(typeShardServer),
		},
		{
			containerName: "mongo-cnf-1",
			want:          "configsvr",
		},
		{
			containerName: "mongo-1-arbiter",
			want:          string(typeShardServer),
		},
		{
			containerName: "standalone",
			want:          "",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range tests {
		port, err := tu.PortForContainer(tc.containerName)
		require.NoError(t, err)

		client := tu.TestClient(ctx, port, t)
		nodeType, err := getClusterRole(ctx, client)
		assert.NoError(t, err)
		assert.Equal(t, tc.want, nodeType, fmt.Sprintf("container name: %s, port: %s", tc.containerName, port))
	}
}

func TestMongosTopologyLabels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := tu.TestClient(ctx, tu.MongoDBStandAlonePort, t)

	ti, err := newTopologyInfo(ctx, client)
	require.NoError(t, err)
	bl := ti.baseLabels()

	assert.Equal(t, "", bl[labelReplicasetName])
	assert.Equal(t, "0", bl[labelReplicasetState])
	assert.Equal(t, "", bl[labelClusterRole])
	assert.Empty(t, bl[labelClusterID]) // this is variable inside a container
}
