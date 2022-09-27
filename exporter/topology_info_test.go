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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestTopologyLabels(t *testing.T) {
	tests := []struct {
		containerName string
		want          map[string]string
	}{
		{
			containerName: "mongos",
			want: map[string]string{
				labelReplicasetName:  "",
				labelReplicasetState: "0",
				labelClusterRole:     "mongos",
				labelClusterID:       "q",
			},
		},
		{
			containerName: "mongo-1-1",
			want: map[string]string{
				labelReplicasetName:  "rs1",
				labelReplicasetState: "1",
				labelClusterRole:     "shardsvr",
				labelClusterID:       "d",
			},
		},
		{
			containerName: "mongo-cnf-1",
			want: map[string]string{
				labelReplicasetName:  "cnf-serv",
				labelReplicasetState: "1",
				labelClusterRole:     "configsvr",
				labelClusterID:       "f",
			},
		},
		{
			containerName: "mongo-1-arbiter",
			want: map[string]string{
				labelReplicasetName:  "rs1",
				labelReplicasetState: "7",
				labelClusterRole:     "shardsvr",
				labelClusterID:       "",
			},
		},
		{
			containerName: "standalone",
			want: map[string]string{
				labelReplicasetName:  "",
				labelReplicasetState: "0",
				labelClusterRole:     "",
				labelClusterID:       "",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range tests {
		port, err := tu.PortForContainer(tc.containerName)
		require.NoError(t, err)

		client := tu.TestClient(ctx, port, t)
		ti := newTopologyInfo(ctx, client, logrus.New())
		bl := ti.baseLabels()
		assert.Equal(t, tc.want[labelReplicasetName], bl[labelReplicasetName], tc.containerName)
		assert.Equal(t, tc.want[labelReplicasetState], bl[labelReplicasetState], tc.containerName)
		assert.Equal(t, tc.want[labelClusterRole], bl[labelClusterRole], tc.containerName)
		if tc.want[labelClusterID] != "" {
			assert.NotEmpty(t, bl[labelClusterID], tc.containerName) // this is variable inside a container
		}
	}
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
