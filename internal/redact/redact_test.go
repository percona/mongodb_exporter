// mongodb_exporter
// Copyright (C) 2025 Percona LLC
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

package redact

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errConnect = errors.New("cannot connect to MongoDB")

type redactTest struct {
	name     string
	uri      string
	expected string
	password string
}

func run(t *testing.T, tests []redactTest) {
	t.Helper()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := MongoURI(test.uri)
			assert.Equal(t, test.expected, actual)
			if test.password != "" {
				assert.NotContains(t, actual, test.password)
			}
		})
	}
}

func TestMongoURI(t *testing.T) {
	t.Parallel()

	//nolint:gosec // the credentials below are test fixtures.
	run(t, []redactTest{
		{
			name:     "user and password",
			uri:      "mongodb://monitor:s3cr3t@127.0.0.1:27017/admin?ssl=true",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin?ssl=true",
			password: "s3cr3t",
		}, {
			name:     "percent-encoded password",
			uri:      "mongodb://monitor:p%40ss%2Fw0rd@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "p%40ss%2Fw0rd",
		}, {
			name:     "srv uri",
			uri:      "mongodb+srv://monitor:s3cr3t@cluster0.example.com/admin?replicaSet=rs1",
			expected: "mongodb+srv://monitor:xxxxx@cluster0.example.com/admin?replicaSet=rs1",
			password: "s3cr3t",
		}, {
			name:     "multiple hosts",
			uri:      "mongodb://monitor:s3cr3t@host1:27017,host2:27017/admin",
			expected: "mongodb://monitor:xxxxx@host1:27017,host2:27017/admin",
			password: "s3cr3t",
		}, {
			name:     "no credentials",
			uri:      "mongodb://127.0.0.1:27017/admin",
			expected: "mongodb://127.0.0.1:27017/admin",
			password: "",
		}, {
			name:     "user without password",
			uri:      "mongodb://monitor@127.0.0.1:27017",
			expected: "mongodb://monitor@127.0.0.1:27017",
			password: "",
		},
	})
}

// TestMongoURIUnparseable covers the inputs url.Parse rejects, where Redacted is unavailable.
func TestMongoURIUnparseable(t *testing.T) {
	t.Parallel()

	run(t, []redactTest{
		{
			// PMM escapes the path to the socket file, which url.Parse rejects.
			name:     "socket path",
			uri:      "mongodb://monitor:s3cr3t@%2Ftmp%2Fmongodb.sock/admin",
			expected: "mongodb://monitor:xxxxx@%2Ftmp%2Fmongodb.sock/admin",
			password: "s3cr3t",
		}, {
			name:     "no scheme",
			uri:      "://///",
			expected: "://///",
			password: "",
		}, {
			name:     "empty",
			uri:      "",
			expected: "",
			password: "",
		},
	})
}

func TestError(t *testing.T) {
	t.Parallel()

	assert.Empty(t, Error(nil))
	assert.Equal(t, errConnect.Error(), Error(errConnect))
}

// TestErrorHidesURIQuotedByParseError checks the leak that makes Error necessary: url.Parse quotes
// the whole URI in the error it returns, so logging that error alone discloses the password.
func TestErrorHidesURIQuotedByParseError(t *testing.T) {
	t.Parallel()

	password := "s3cr3t"
	socket := "%2Ftmp%2Fmongodb.sock"

	_, err := url.Parse("mongodb://monitor:" + password + "@" + socket + "/admin")
	require.Error(t, err)
	require.Contains(t, err.Error(), password)

	redacted := Error(err)
	assert.NotContains(t, redacted, password)
	assert.Contains(t, redacted, "mongodb://monitor:xxxxx@"+socket+"/admin")
}
