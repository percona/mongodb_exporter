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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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
			cnxOpts := ConnectionParams{
				URI:           dsn,
				AuthMechanism: AuthMechanismDefault,
				DirectConnect: UseDirecConnection,
			}
			client, err := connect(ctx, cnxOpts)
			assert.NoError(t, err, name)
			err = client.Disconnect(ctx)
			assert.NoError(t, err, name)
		}
	})

	//nolint:dupl
	t.Run("Test per-request connection", func(t *testing.T) {
		log := logrus.New()

		exporterOpts := &Opts{
			Logger: log,
			ConnectionParams: ConnectionParams{
				URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
				GlobalConnPool: false,
			},
		}

		e, err := New(exporterOpts)
		if err != nil {
			log.Fatal(err)
		}

		ts := httptest.NewServer(e.handler())
		defer ts.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := http.Get(ts.URL) //nolint:noctx
				assert.Nil(t, e.client)
				assert.NoError(t, err)
				g, err := ioutil.ReadAll(res.Body)
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
			Logger: log,
			ConnectionParams: ConnectionParams{
				URI:            fmt.Sprintf("mongodb://127.0.0.1:%s/admin", tu.MongoDBS1PrimaryPort),
				GlobalConnPool: true,
			},
		}

		e, err := New(exporterOpts)
		if err != nil {
			log.Fatal(err)
		}

		ts := httptest.NewServer(e.handler())
		defer ts.Close()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := http.Get(ts.URL) //nolint:noctx
				assert.NotNil(t, e.client)
				assert.NoError(t, err)
				g, err := ioutil.ReadAll(res.Body)
				_ = res.Body.Close()
				assert.NoError(t, err)
				assert.NotEmpty(t, g)
			}()
		}

		wg.Wait()
	})
}

func TestSSL(t *testing.T) {
	// See https://hub.docker.com/r/rzhilkibaev/mongo-x509-auth-ssl/ for an example container with SSL enabled
	t.Skip("This test requires an special environment")

	t.Run("Connect SSL fail when verifying self signed certs", func(t *testing.T) {
		ctx := context.TODO()

		dsn := fmt.Sprintf("mongodb://127.0.0.1:27017/?tlsCAFile=%s&tlsCertificateKeyFile=%s&", "/tmp/mongodb-CA.pem", "/tmp/mongodb-client.pem")
		opts := ConnectionParams{
			AuthMechanism:      AuthMechanismX509,
			URI:                dsn,
			DirectConnect:      UseDirecConnection,
			InsecureSkipVerify: false,
			ConnectTimeout:     1 * time.Second,
		}
		_, err := connect(ctx, opts)
		assert.Error(t, err)
	})

	t.Run("Connect SSL", func(t *testing.T) {
		ctx := context.TODO()

		dsn := fmt.Sprintf("mongodb://127.0.0.1:27017/?tlsCAFile=%s&tlsCertificateKeyFile=%s&", "/tmp/mongodb-CA.pem", "/tmp/mongodb-client.pem")
		opts := ConnectionParams{
			AuthMechanism:      AuthMechanismX509,
			URI:                dsn,
			DirectConnect:      UseDirecConnection,
			InsecureSkipVerify: true,
			ConnectTimeout:     1 * time.Second,
		}
		client, err := connect(ctx, opts)
		assert.NoError(t, err)
		err = client.Disconnect(ctx)
		assert.NoError(t, err)
	})
}
