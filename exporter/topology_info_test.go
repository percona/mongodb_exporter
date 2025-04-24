// mongodb_exporter
// Copyright (C) 2017 Percona LLC
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
	"testing"
	"time"

	"github.com/prometheus/common/promslog"
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
				labelReplicasetState: "",
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
				labelReplicasetState: "",
				labelClusterRole:     "",
				labelClusterID:       "",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range tests {
		t.Run(tc.containerName, func(t *testing.T) {
			port, err := tu.PortForContainer(tc.containerName)
			require.NoError(t, err)

			client := tu.TestClient(ctx, port, t)
			ti := newTopologyInfo(ctx, client, promslog.New(&promslog.Config{}))
			bl := ti.baseLabels()
			assert.Equal(t, tc.want[labelReplicasetName], bl[labelReplicasetName], tc.containerName)
			assert.Equal(t, tc.want[labelReplicasetState], bl[labelReplicasetState], tc.containerName)
			assert.Equal(t, tc.want[labelClusterRole], bl[labelClusterRole], tc.containerName)
			if tc.want[labelClusterID] != "" {
				assert.NotEmpty(t, bl[labelClusterID], tc.containerName) // this is variable inside a container
			}
		})
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
		logger := promslog.New(&promslog.Config{}).With("component", "test")
		nodeType, err := getClusterRole(ctx, client, logger)
		assert.NoError(t, err)
		assert.Equal(t, tc.want, nodeType, fmt.Sprintf("container name: %s, port: %s", tc.containerName, port))
	}
}
