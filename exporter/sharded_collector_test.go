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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:paralleltest
func TestShardedCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClientMongoS(ctx, t)
	c := newShardedCollector(ctx, client, logrus.New(), false)

	expected := strings.NewReader(`
	# HELP mongodb_sharded_collection_chunks_count sharded collection chunks.
	# TYPE mongodb_sharded_collection_chunks_count counter
    mongodb_sharded_collection_chunks_count{collection="system.sessions",database="config",shard="rs2"} 1024` + "\n")
	filter := []string{
		"mongodb_sharded_collection_chunks_count",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
