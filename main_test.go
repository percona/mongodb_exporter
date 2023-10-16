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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestBuildExporter(t *testing.T) {
	opts := GlobalFlags{
		CollStatsNamespaces:   "c1,c2,c3",
		IndexStatsCollections: "i1,i2,i3",
		GlobalConnPool:        false, // to avoid testing the connection
		WebListenAddress:      "localhost:12345",
		WebTelemetryPath:      "/mymetrics",
		LogLevel:              "debug",

		EnableDiagnosticData:   true,
		EnableReplicasetStatus: true,

		CompatibleMode: true,
	}
	log := logrus.New()
	buildExporter(opts, "mongodb://usr:pwd@127.0.0.1/", log)
}

func TestBuildURI(t *testing.T) {
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
	}
	for _, tc := range tests {
		newUri := buildURI(tc.origin, tc.newUser, tc.newPassword)
		// t.Logf("Origin: %s", tc.origin)
		// t.Logf("Expect: %s", tc.expect)
		// t.Logf("Result: %s", newUri)
		assert.Equal(t, newUri, tc.expect)
	}
}
