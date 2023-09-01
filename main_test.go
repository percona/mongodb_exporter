// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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

	t.Log("\nuri with prefix and auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixAuthURI, newUser, newPass)
	t.Logf("Origin: %s", originalPrefixAuthURI)
	t.Logf("Expect: %s", originalPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalPrefixAuthURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with prefix and auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixAuthURI, "", "")
	t.Logf("Origin: %s", originalPrefixAuthURI)
	t.Logf("Expect: %s", originalPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalPrefixAuthURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with no prefix and auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalAuthURI, newUser, newPass)
	t.Logf("Origin: %s", originalAuthURI)
	t.Logf("Expect: %s", originalAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalAuthURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with no prefix and auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalAuthURI, "", "")
	t.Logf("Origin: %s", originalAuthURI)
	t.Logf("Expect: %s", originalAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalAuthURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with prefix and no auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixBareURI, newUser, newPass)
	t.Logf("Origin: %s", originalPrefixBareURI)
	t.Logf("Expect: %s", changedPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != changedPrefixAuthURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with prefix and no auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalPrefixBareURI, "", "")
	t.Logf("Origin: %s", originalPrefixBareURI)
	t.Logf("Expect: %s", originalPrefixBareURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalPrefixBareURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with no prefix and no auth, and auth supplied in opt.User/Password")
	newUri = buildURI(originalBareURI, newUser, newPass)
	t.Logf("Origin: %s", originalBareURI)
	t.Logf("Expect: %s", changedPrefixAuthURI)
	t.Logf("Result: %s", newUri)
	if newUri != changedPrefixAuthURI {
		t.Fail()
	}
	newUri = ""

	t.Log("\nuri with no prefix and no auth, no auth supplied in opt.User/Password")
	newUri = buildURI(originalBareURI, "", "")
	t.Logf("Origin: %s", originalBareURI)
	t.Logf("Expect: %s", originalBareURI)
	t.Logf("Result: %s", newUri)
	if newUri != originalBareURI {
		t.Fail()
	}
	newUri = ""

}
