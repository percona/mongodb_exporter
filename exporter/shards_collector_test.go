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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:paralleltest
func TestShardsCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := tu.DefaultTestClientMongoS(ctx, t)
	shardTestCollection(ctx, t, client, "test", "shard")
	c := newShardsCollector(ctx, client, promslog.New(&promslog.Config{}), false)

	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(c); err != nil {
		panic(fmt.Errorf("registering collector failed: %w", err))
	}

	expected := []map[string]string{
		{"collection": "shard", "database": "test", "shard": "rs1"},
		{"collection": "shard", "database": "test", "shard": "rs2"},
	}

	got, err := reg.Gather()
	assert.NoError(t, err)
	res := []map[string]string{}
	for _, r := range got {
		if r.GetName() != "mongodb_shards_collection_chunks_count" {
			continue
		}
		for _, m := range r.Metric {
			row := make(map[string]string)
			for _, l := range m.GetLabel() {
				row[l.GetName()] = l.GetValue()
			}

			res = append(res, row)
		}
	}
	for _, v := range expected {
		assert.Contains(t, res, v)
	}
}

func TestShardsCollectorGetInfoForChunk(t *testing.T) {
	t.Parallel()

	c := &shardsCollector{base: newBaseCollector(nil, promslog.New(&promslog.Config{}))}
	labels, chunks, ok := c.getInfoForChunk(bson.M{"shard": "rs1", "nChunks": int32(2)}, "test", "test.shard")

	assert.True(t, ok)
	assert.Equal(t, int32(2), chunks)
	assert.Equal(t, map[string]string{"database": "test", "collection": "shard", "shard": "rs1"}, labels)

	_, _, ok = c.getInfoForChunk(bson.M{"dropped": true}, "test", "test.shard")
	assert.False(t, ok)

	_, _, ok = c.getInfoForChunk(bson.M{"nChunks": int32(2)}, "test", "test.shard")
	assert.False(t, ok)
}
