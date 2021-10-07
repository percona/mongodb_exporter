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

func TestMongosTopologyLabels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := tu.TestClient(ctx, tu.MongoDBStandAlonePort, t)

	ti, err := newTopologyInfo(ctx, client)
	require.NoError(t, err)
	bl := ti.baseLabels()

	assert.Equal(t, "", bl[labelReplicasetName])
	assert.Equal(t, "0", bl[labelReplicasetState])
	assert.Equal(t, "mongod", bl[labelClusterRole])
	assert.Empty(t, bl[labelClusterID]) // this is variable inside a container
}
