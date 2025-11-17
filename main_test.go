// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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

package main

import (
	"net"
	"strings"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/exporter"
	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestParseURIList(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"mongodb://server": {"mongodb://server"},
		"mongodb+srv://server1,server2,mongodb://server3,server4,server5": {
			"mongodb+srv://server1",
			"mongodb://server2",
			"mongodb://server3,server4,server5",
		},
		"server1": {"mongodb://server1"},
		"server1,server2,server3": {
			"mongodb://server1",
			"mongodb://server2",
			"mongodb://server3",
		},
		"mongodb.server,server2": {
			"mongodb://mongodb.server",
			"mongodb://server2",
		},
		"standalone,mongodb://server1,server2,mongodb+srv://server3,server4,mongodb://server5": {
			"mongodb://standalone",
			"mongodb://server1,server2",
			"mongodb+srv://server3",
			"mongodb://server4",
			"mongodb://server5",
		},
	}
	logger := promslog.New(&promslog.Config{})
	for test, expected := range tests {
		actual := exporter.ParseURIList(strings.Split(test, ","), logger, false)
		assert.Equal(t, expected, actual)
	}
}

func TestSplitCluster(t *testing.T) {
	// Can't run in parallel because it patches the net.DefaultResolver

	tests := map[string][]string{
		"mongodb://server": {"mongodb://server"},
		"mongodb://user:pass@server1,server2/admin?replicaSet=rs1,mongodb://server3,server4,server5": {
			"mongodb://user:pass@server1/admin?replicaSet=rs1",
			"mongodb://user:pass@server2/admin?replicaSet=rs1",
			"mongodb://server3",
			"mongodb://server4",
			"mongodb://server5",
		},
		"mongodb://server1,mongodb://user:pass@server2,server3?arg=1&arg2=2,mongodb+srv://user:pass@server.example.com/db?replicaSet=rs1": {
			"mongodb://server1",
			"mongodb://user:pass@server2?arg=1&arg2=2",
			"mongodb://user:pass@server3?arg=1&arg2=2",
			"mongodb://user:pass@mongo1.example.com:17001/db?authSource=admin&replicaSet=rs1",
			"mongodb://user:pass@mongo2.example.com:17002/db?authSource=admin&replicaSet=rs1",
			"mongodb://user:pass@mongo3.example.com:17003/db?authSource=admin&replicaSet=rs1",
		},
	}

	logger := promslog.New(&promslog.Config{})

	srv := tu.SetupFakeResolver()

	defer func(t *testing.T) {
		t.Helper()
		err := srv.Close()
		assert.NoError(t, err)
	}(t)
	defer mockdns.UnpatchNet(net.DefaultResolver)

	for test, expected := range tests {
		actual := exporter.ParseURIList(strings.Split(test, ","), logger, true)
		assert.Equal(t, expected, actual)
	}
}

func TestBuildExporter(t *testing.T) {
	t.Parallel()
	opts := GlobalFlags{
		CollStatsNamespaces:   "c1,c2,c3",
		IndexStatsCollections: "i1,i2,i3",
		GlobalConnPool:        false, // to avoid testing the connection
		WebListenAddress:      "localhost:12345",
		WebTelemetryPath:      "/mymetrics",
		LogLevel:              "debug",

		EnableDiagnosticData:   true,
		EnableReplicasetStatus: true,
		EnableReplicasetConfig: true,

		CompatibleMode: true,
	}
	log := promslog.New(&promslog.Config{})
	buildExporter(buildOpts(opts), "mongodb://usr:pwd@127.0.0.1/", log)
}

func TestBuildURI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		situation   string
		origin      string
		newUser     string
		newPassword string
		expect      string
	}{
		{
			situation:   "uri with prefix and auth, and auth supplied in opt.User/Password",
			origin:      "mongodb://usr:pwd@127.0.0.1",
			newUser:     "xxx",
			newPassword: "yyy",
			expect:      "mongodb://usr:pwd@127.0.0.1",
		},
		{
			situation:   "uri with prefix and auth, no auth supplied in opt.User/Password",
			origin:      "mongodb://usr:pwd@127.0.0.1",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb://usr:pwd@127.0.0.1",
		},
		{
			situation:   "uri with no prefix and auth, and auth supplied in opt.User/Password",
			origin:      "usr:pwd@127.0.0.1",
			newUser:     "xxx",
			newPassword: "yyy",
			expect:      "mongodb://usr:pwd@127.0.0.1",
		},
		{
			situation:   "uri with no prefix and auth, no auth supplied in opt.User/Password",
			origin:      "usr:pwd@127.0.0.1",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb://usr:pwd@127.0.0.1",
		},
		{
			situation:   "uri with prefix and no auth, and auth supplied in opt.User/Password",
			origin:      "mongodb://127.0.0.1",
			newUser:     "xxx",
			newPassword: "yyy",
			expect:      "mongodb://xxx:yyy@127.0.0.1",
		},
		{
			situation:   "uri with prefix and no auth, no auth supplied in opt.User/Password",
			origin:      "mongodb://127.0.0.1",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb://127.0.0.1",
		},
		{
			situation:   "uri with no prefix and no auth, and auth supplied in opt.User/Password",
			origin:      "127.0.0.1",
			newUser:     "xxx",
			newPassword: "yyy",
			expect:      "mongodb://xxx:yyy@127.0.0.1",
		},
		{
			situation:   "uri with no prefix and no auth, no auth supplied in opt.User/Password",
			origin:      "127.0.0.1",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb://127.0.0.1",
		},
		{
			situation:   "uri with no prefix and no auth, auth supplied in opt.User/Password, and user prefixed with mongodb",
			origin:      "127.0.0.1",
			newUser:     "mongodbxxx",
			newPassword: "yyy",
			expect:      "mongodb://mongodbxxx:yyy@127.0.0.1",
		},
		{
			situation:   "uri with prefix and no auth, auth supplied in opt.User/Password, and user prefixed with mongodb",
			origin:      "mongodb://127.0.0.1",
			newUser:     "mongodbxxx",
			newPassword: "yyy",
			expect:      "mongodb://mongodbxxx:yyy@127.0.0.1",
		},
		{
			situation:   "uri with srv prefix and no auth, auth supplied in opt.User/Password, and user prefixed with mongodb",
			origin:      "mongodb+srv://127.0.0.1",
			newUser:     "mongodbxxx",
			newPassword: "yyy",
			expect:      "mongodb+srv://mongodbxxx:yyy@127.0.0.1",
		},
		{
			situation:   "uri with srv prefix and auth, auth supplied in opt.User/Password, and user prefixed with mongodb",
			origin:      "mongodb+srv://xxx:zzz@127.0.0.1",
			newUser:     "mongodbxxx",
			newPassword: "yyy",
			expect:      "mongodb+srv://xxx:zzz@127.0.0.1",
		},
		{
			situation:   "uri with srv prefix and auth, no auth supplied in opt.User/Password, and user prefixed with mongodb",
			origin:      "mongodb+srv://xxx:zzz@127.0.0.1",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb+srv://xxx:zzz@127.0.0.1",
		},
		{
			situation:   "url with special characters in username and password",
			origin:      "mongodb://127.0.0.1",
			newUser:     "xxx?!#$%^&*()_+",
			newPassword: "yyy?!#$%^&*()_+",
			expect:      "mongodb://xxx%3F%21%23$%25%5E&%2A%28%29_+:yyy%3F%21%23$%25%5E&%2A%28%29_+@127.0.0.1",
		},
		{
			situation:   "path to socket",
			origin:      "mongodb:///tmp/mongodb-27017.sock",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb:///tmp/mongodb-27017.sock",
		},
		{
			situation:   "path to socket with params",
			origin:      "mongodb://username:s3cur3%20p%40$$w0r4.@%2Fvar%2Frun%2Fmongodb%2Fmongodb.sock/database?connectTimeoutMS=1000&directConnection=true&serverSelectionTimeoutMS=1000",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb://username:s3cur3%20p%40$$w0r4.@%2Fvar%2Frun%2Fmongodb%2Fmongodb.sock/database?connectTimeoutMS=1000&directConnection=true&serverSelectionTimeoutMS=1000",
		},
		{
			situation:   "path to socket with auth",
			origin:      "mongodb://xxx:yyy@/tmp/mongodb-27017.sock",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb://xxx:yyy@/tmp/mongodb-27017.sock",
		},
		{
			situation:   "path to socket with auth and user params",
			origin:      "mongodb:///tmp/mongodb-27017.sock",
			newUser:     "xxx",
			newPassword: "yyy",
			expect:      "mongodb://xxx:yyy@/tmp/mongodb-27017.sock",
		},
		{
			situation:   "path to socket without prefix",
			origin:      "/tmp/mongodb-27017.sock",
			newUser:     "",
			newPassword: "",
			expect:      "mongodb:///tmp/mongodb-27017.sock",
		},
		{
			situation:   "path to socket without prefix with auth",
			origin:      "/tmp/mongodb-27017.sock",
			newUser:     "xxx",
			newPassword: "yyy",
			expect:      "mongodb://xxx:yyy@/tmp/mongodb-27017.sock",
		},
	}
	for _, tc := range tests {
		t.Run(tc.situation, func(t *testing.T) {
			newURI := exporter.BuildURI(tc.origin, tc.newUser, tc.newPassword)
			assert.Equal(t, tc.expect, newURI)
		})
	}
}
