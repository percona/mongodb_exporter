// mongodb_exporter
// Copyright (C) 2024	 Percona LLC
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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestPBMCollector(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	port, err := tu.PortForContainer("standalone-backup")
	require.NoError(t, err)
	client := tu.TestClient(ctx, port, t)
	mongoURI := "mongodb://pbm:pbm@localhost:27037" //nolint:gosec

	c := newPbmCollector(ctx, client, mongoURI, logrus.New())

	filter := []string{
		"mongodb_pbm_cluster_backup_configured",
		"mongodb_pbm_agent_status",
	}
	count := testutil.CollectAndCount(c, filter...)
	assert.Equal(t, len(filter), count, "PBM metrics are missing")

	expected := strings.NewReader(`
	# HELP mongodb_pbm_agent_status PBM Agent Status
	# TYPE mongodb_pbm_agent_status gauge
	mongodb_pbm_agent_status{host="192.168.167.3:27017",replica_set="standaloneBackup",role="P"} 0
	# HELP mongodb_pbm_cluster_backup_configured PBM backups are configured for the cluster
	# TYPE mongodb_pbm_cluster_backup_configured gauge
	mongodb_pbm_cluster_backup_configured 1
`)
	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
