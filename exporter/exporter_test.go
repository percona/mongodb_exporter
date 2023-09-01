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
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

// Use this for testing because labels like cluster ID are not constant in docker containers
// so we cannot use the real topology labels in tests.
type labelsGetterMock struct{}

func (l labelsGetterMock) baseLabels() map[string]string {
	return map[string]string{}
}

func (l labelsGetterMock) loadLabels(context.Context) error {
	return nil
}

//nolint:funlen
func TestConnect(t *testing.T) {
	hostname := "127.0.0.1"
	ctx := context.Background()

	ports := map[string]string{
		"standalone":          tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017"),
		"shard-1 primary":     tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"),
		"shard-1 secondary-1": tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "17002"),
		"shard-1 secondary-2": tu.GetenvDefault("TEST_MONGODB_S1_SECONDARY2_PORT", "17003"),
		"shard-2 primary":     tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"),
		"shard-2 secondary-1": tu.GetenvDefault("TEST_MONGODB_S2_SECONDARY1_PORT", "17005"),
		"shard-2 secondary-2": tu.GetenvDefault("TEST_MONGODB_S2_SECONDARY2_PORT", "17006"),
		"config server 1":     tu.GetenvDefault("TEST_MONGODB_CONFIGSVR1_PORT", "17007"),
		"mongos":              tu.GetenvDefault("TEST_MONGODB_MONGOS_PORT", "17000"),
	}

	t.Run("Connect without SSL", func(t *testing.T) {
		for name, port := range ports {
			dsn := fmt.Sprintf("mongodb://%s:%s/admin", hostname, port)
			client, err := connect(ctx, dsn, true)
			assert.NoError(t, err, name)
			err = client.Disconnect(ctx)
			assert.NoError(t, err, name)
		}
	})

	//nolint:dupl
	t.Run("Test per-request connection", func(t *testing.T) {
		log := logrus.New()

		exporterOpts := &Opts{
			Logger:         log,
			URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
			GlobalConnPool: false,
			DirectConnect:  true,
		}

		e := New(exporterOpts)

		ts := httptest.NewServer(e.Handler())
		defer ts.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := http.Get(ts.URL) //nolint:noctx
				assert.Nil(t, e.client)
				assert.NoError(t, err)
				g, err := io.ReadAll(res.Body)
				_ = res.Body.Close()
				assert.NoError(t, err)
				assert.NotEmpty(t, g)
			}()
		}

		wg.Wait()
	})

	//nolint:dupl
	t.Run("Test global connection", func(t *testing.T) {
		log := logrus.New()

		exporterOpts := &Opts{
			Logger:         log,
			URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
			GlobalConnPool: true,
			DirectConnect:  true,
		}

		e := New(exporterOpts)

		ts := httptest.NewServer(e.Handler())
		defer ts.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := http.Get(ts.URL) //nolint:noctx
				assert.NotNil(t, e.client)
				assert.NoError(t, err)
				g, err := io.ReadAll(res.Body)
				_ = res.Body.Close()
				assert.NoError(t, err)
				assert.NotEmpty(t, g)
			}()
		}

		wg.Wait()
	})
}

// How this test works?
// When connected to a MongoS instance, the makeRegistry method should skip
// adding replSetGetStatusCollector. To test that, we try to unregister a
// replSetGetStatusCollector and it should return false since it wasn't registered.
// Note: Two Collectors are considered equal if their Describe method yields the
// same set of descriptors.
// unregister will try to Describe to get the descriptors set and we are using
// DescribeByCollect so, in the logs, you will see an error:
// msg="cannot get replSetGetStatus: replSetGetStatus is not supported through mongos"
// This is correct. Collect is being executed to Describe and Unregister.
func TestMongoS(t *testing.T) {
	hostname := "127.0.0.1"
	ctx := context.Background()

	tests := []struct {
		port string
		want bool
	}{
		{
			port: tu.GetenvDefault("TEST_MONGODB_MONGOS_PORT", "17000"),
			want: false,
		},
		{
			port: tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"),
			want: true,
		},
	}

	for _, test := range tests {
		dsn := fmt.Sprintf("mongodb://%s:%s/admin", hostname, test.port)
		client, err := connect(ctx, dsn, true)
		assert.NoError(t, err)

		exporterOpts := &Opts{
			Logger:                 logrus.New(),
			URI:                    dsn,
			GlobalConnPool:         false,
			EnableReplicasetStatus: true,
		}

		e := New(exporterOpts)

		rsgsc := newReplicationSetStatusCollector(ctx, client, e.opts.Logger,
			e.opts.CompatibleMode, new(labelsGetterMock))

		r := e.makeRegistry(ctx, client, new(labelsGetterMock), *e.opts)

		res := r.Unregister(rsgsc)
		assert.Equal(t, test.want, res, fmt.Sprintf("Port: %v", test.port))
		err = client.Disconnect(ctx)
		assert.NoError(t, err)
	}
}

func TestMongoUp(t *testing.T) {
	ctx := context.Background()

	dsn := "mongodb://127.0.0.1:123456/admin"
	client, err := connect(ctx, dsn, true)
	assert.Error(t, err)

	exporterOpts := &Opts{
		Logger:         logrus.New(),
		URI:            dsn,
		GlobalConnPool: false,
		CollectAll:     true,
	}

	e := New(exporterOpts)

	gc := newGeneralCollector(ctx, client, e.opts.Logger)

	r := e.makeRegistry(ctx, client, new(labelsGetterMock), *e.opts)

	res := r.Unregister(gc)
	assert.Equal(t, true, res)
}
