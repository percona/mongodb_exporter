// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactMongoUri(t *testing.T) {
	uri := "mongodb://mongodb_exporter:s3cr3tpassw0rd@localhost:27017"
	expected := "mongodb://****:****@localhost:27017"
	actual := RedactMongoUri(uri)
	if expected != actual {
		t.Errorf("%q != %q", expected, actual)
	}
}

func TestMongoSession(t *testing.T) {
	mso := &MongoSessionOpts{
		URI: "mongodb://localhost:27017",
	}
	session := MongoClient(mso)
	require.NotNil(t, session)
	if session == nil {
		t.Error("session is nil")
	}
	serverVersion, err := MongoSessionServerVersion(session)
	assert.NoError(t, err)
	assert.NotEmpty(t, serverVersion)

	nodeType, err := MongoSessionNodeType(session)
	assert.NoError(t, err)
	assert.Equal(t, "mongod", nodeType)
}

func TestTestConnection(t *testing.T) {
	mso := MongoSessionOpts{
		URI: "mongodb://localhost:27017",
	}
	_, err := TestConnection(mso)
	require.NoError(t, err)
}

func TestTestSSLConnection(t *testing.T) {
	tlsCertificateKeyFile := "../testdata/client.pem"
	tlsCAFile := "../testdata/ca.crt"

	mso := MongoSessionOpts{
		URI: "mongodb://127.0.0.1:27017/admin/?ssl=true&tlsCertificateKeyFile=" + tlsCertificateKeyFile + "&tlsCAFile=" + tlsCAFile + "&tlsInsecure=true&serverSelectionTimeoutMS=2000",
	}
	_, err := TestConnection(mso)
	require.NoError(t, err)
}
