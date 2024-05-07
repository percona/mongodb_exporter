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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

//nolint:paralleltest
func TestChunksCollector(t *testing.T) {
	t.Skip("This is failing in GitHub actions. Shards are not ready yet")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClientMongoS(ctx, t)
	c := newChunksCollector(ctx, client, logrus.New(), false)

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
