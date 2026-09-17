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

//go:build gssapi

package exporter

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

// GSSAPI is only compiled into the driver with -tags gssapi, which also needs the Kerberos
// development headers present at build time. Without the tag the driver answers every GSSAPI
// handshake with "GSSAPI support not enabled during build", so this file is built only when
// the tag is set -- as every make test target and CI set it.

func generateKerberosConfigFile(t *testing.T) *os.File {
	t.Helper()
	kerberosHost, err := tu.IPForContainer("kerberos")
	require.NoError(t, err)

	config := fmt.Sprintf(`
[libdefaults]
    default_realm = PERCONATEST.COM
    forwardable = true
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ignore_acceptor_hostname = true
    rdns = false
[realms]
    PERCONATEST.COM = {
        kdc_ports = 88
        kdc = %s
    }
[domain_realm]
    .perconatest.com = PERCONATEST.COM
    perconatest.com = PERCONATEST.COM
    %s = PERCONATEST.COM
`, kerberosHost, kerberosHost)
	configFile, err := os.Create(t.TempDir() + "/krb5.conf")
	require.NoError(t, err)

	_, err = configFile.WriteString(config)
	require.NoError(t, err)

	return configFile
}

func TestGSSAPIAuth(t *testing.T) {
	logger := promslog.New(&promslog.Config{})

	mongoHost, err := tu.IPForContainer("psmdb-kerberos")
	require.NoError(t, err)

	configFile := generateKerberosConfigFile(t)
	require.NoError(t, err)
	defer func() {
		_ = configFile.Close()
		t.Setenv("KRB5_CONFIG", "")
	}()

	t.Setenv("KRB5_CONFIG", configFile.Name())
	ctx := context.Background()

	username := "pmm-test%40PERCONATEST.COM"
	password := "password1"
	uri := fmt.Sprintf("mongodb://%s:%s@%s/?authSource=$external&authMechanism=GSSAPI",
		username,
		password,
		net.JoinHostPort(mongoHost, "27017"),
	)
	exporterOpts := &Opts{
		URI:            uri,
		Logger:         logger,
		CollectAll:     true,
		GlobalConnPool: false,
		DirectConnect:  true,
	}

	client, err := connect(ctx, exporterOpts)
	assert.NoError(t, err)

	e := New(exporterOpts)
	nodeType, _ := getNodeType(ctx, client)
	gc := newGeneralCollector(ctx, client, nodeType, e.opts.Logger)
	r := e.makeRegistry(ctx, client, new(labelsGetterMock), *e.opts)

	expected := strings.NewReader(`
		# HELP mongodb_up Whether MongoDB is up.
		# TYPE mongodb_up gauge
		mongodb_up {cluster_role="mongod"} 1` + "\n")

	filter := []string{
		"mongodb_up",
	}
	err = testutil.CollectAndCompare(gc, expected, filter...)
	require.NoError(t, err, "mongodb_up metric should be 1")

	res := r.Unregister(gc)
	assert.True(t, res)
}
