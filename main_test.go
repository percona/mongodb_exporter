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
)

func TestBuildExporter(t *testing.T) {
	opts := GlobalFlags{
		CollStatsNamespaces:   "c1,c2,c3",
		IndexStatsCollections: "i1,i2,i3",
		URI:                   "mongodb://usr:pwd@127.0.0.1/",
		GlobalConnPool:        false, // to avoid testing the connection
		WebListenAddress:      "localhost:12345",
		WebTelemetryPath:      "/mymetrics",
		LogLevel:              "debug",

		EnableDiagnosticData:   true,
		EnableReplicasetStatus: true,

		CompatibleMode: true,
	}

	buildExporter(opts)
}

func TestBuildURI(t *testing.T) {
	const newUser = "xxx"
	const newPass = "yyy"

	const originalBareURI = "127.0.0.1"
	const originalAuthURI = "usr:pwd@127.0.0.1"

	const originalPrefixBareURI = "mongodb://127.0.0.1"
	const originalPrefixAuthURI = "mongodb://usr:pwd@127.0.0.1"
	const changedPrefixAuthURI = "mongodb://xxx:yyy@127.0.0.1"

	var newUri string
	resetNewUri := func() {
		newUri = ""
	}

	t.Log("\nuri with prefix and auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixAuthURI, newUser, newPass)
	t.Logf("Origin: %s", originalPrefixAuthURI)
	t.Logf("Expect: %s", originalPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalPrefixAuthURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with prefix and auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixAuthURI, "", "")
	t.Logf("Origin: %s", originalPrefixAuthURI)
	t.Logf("Expect: %s", originalPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalPrefixAuthURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with no prefix and auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalAuthURI, newUser, newPass)
	t.Logf("Origin: %s", originalAuthURI)
	t.Logf("Expect: %s", originalAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalAuthURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with no prefix and auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalAuthURI, "", "")
	t.Logf("Origin: %s", originalAuthURI)
	t.Logf("Expect: %s", originalAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalAuthURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with prefix and no auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixBareURI, newUser, newPass)
	t.Logf("Origin: %s", originalPrefixBareURI)
	t.Logf("Expect: %s", changedPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != changedPrefixAuthURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with prefix and no auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixBareURI, "", "")
	t.Logf("Origin: %s", originalPrefixBareURI)
	t.Logf("Expect: %s", originalPrefixBareURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalPrefixBareURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with no prefix and no auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalBareURI, newUser, newPass)
	t.Logf("Origin: %s", originalBareURI)
	t.Logf("Expect: %s", changedPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != changedPrefixAuthURI {
		t.Fail()
	}
	resetNewUri()

	t.Log("\nuri with no prefix and no auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalBareURI, "", "")
	t.Logf("Origin: %s", originalBareURI)
	t.Logf("Expect: %s", originalBareURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalBareURI {
		t.Fail()
	}
	resetNewUri()
}
