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
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errConnect = errors.New("cannot connect to MongoDB")

// specialChars exercises the punctuation MongoDB permits in a password. Several of these bytes are
// not representable in a URI unescaped, so they are also what drives url.Parse into the error path
// that MongoURI has to cover on its own.
const specialChars = `-=^?$&%#_<>*()`

// specialCharsKeptByParseError is specialChars without "?" and "#". Those two end the authority, so
// url.Parse cuts the URI at them and the error it returns quotes only the part before, which leaves
// nothing for Error to anchor on. See TestErrorLeavesTruncatedURIAlone.
const specialCharsKeptByParseError = `-=^$&%_<>*()`

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

// TestMongoURI covers the URIs url.Parse accepts, where Redacted does the work.
func TestMongoURI(t *testing.T) {
	t.Parallel()

	//nolint:gosec // the credentials below are test fixtures.
	run(t, []redactTest{
		{
			name:     "simple password",
			uri:      "mongodb://monitor:s3cr3t@127.0.0.1:27017/admin?ssl=true",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin?ssl=true",
			password: "s3cr3t",
		}, {
			name:     "sub-delimiters",
			uri:      "mongodb://monitor:$&+,;=@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "$&+,;=",
		}, {
			// The password separator itself. url.Parse splits userinfo on the first colon, so the
			// rest stays in the password.
			name:     "colon in password",
			uri:      "mongodb://monitor:pa:ss@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "pa:ss",
		}, {
			// The userinfo separator. url.Parse anchors on the last @, so this still parses.
			name:     "at sign in password",
			uri:      "mongodb://monitor:pa@ss@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "pa@ss",
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
			uri:      "mongodb://monitor:$&+,;=@host1:27017,host2:27017/admin",
			expected: "mongodb://monitor:xxxxx@host1:27017,host2:27017/admin",
			password: "$&+,;=",
		},
	})
}

// TestMongoURIUnparseable covers the inputs url.Parse rejects, where Redacted is unavailable and
// the fallback expression has to find the credentials on its own. Every password here contains a
// byte that is legal in MongoDB but not in a URI.
func TestMongoURIUnparseable(t *testing.T) {
	t.Parallel()

	//nolint:gosec // the credentials below are test fixtures.
	run(t, []redactTest{
		{
			name:     "special characters",
			uri:      "mongodb://monitor:" + specialChars + "@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: specialChars,
		}, {
			name:     "slash in password",
			uri:      "mongodb://monitor:pa/ss@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "pa/ss",
		}, {
			name:     "question mark in password",
			uri:      "mongodb://monitor:pa?ss@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "pa?ss",
		}, {
			name:     "hash in password",
			uri:      "mongodb://monitor:pa#ss@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "pa#ss",
		}, {
			name:     "space in password",
			uri:      "mongodb://monitor:pa ss@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "pa ss",
		}, {
			name:     "quotes in password",
			uri:      "mongodb://monitor:p'\"s@127.0.0.1:27017/admin",
			expected: "mongodb://monitor:xxxxx@127.0.0.1:27017/admin",
			password: "p'\"s",
		}, {
			name:     "special characters over srv",
			uri:      "mongodb+srv://monitor:" + specialChars + "@cluster0.example.com/admin?replicaSet=rs1",
			expected: "mongodb+srv://monitor:xxxxx@cluster0.example.com/admin?replicaSet=rs1",
			password: specialChars,
		}, {
			name:     "special characters over multiple hosts",
			uri:      "mongodb://monitor:" + specialChars + "@host1:27017,host2:27017/admin",
			expected: "mongodb://monitor:xxxxx@host1:27017,host2:27017/admin",
			password: specialChars,
		}, {
			// PMM escapes the path to the socket file, which url.Parse rejects.
			name:     "socket path",
			uri:      "mongodb://monitor:" + specialChars + "@%2Ftmp%2Fmongodb.sock/admin",
			expected: "mongodb://monitor:xxxxx@%2Ftmp%2Fmongodb.sock/admin",
			password: specialChars,
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

// TestMongoURICredentialsHiddenByCleanParse covers the case that rules out shortcutting on a
// successful parse: a password beginning with digits and containing a slash leaves url.Parse with a
// valid host and port, so it reports no userinfo at all while the credentials sit in the path.
func TestMongoURICredentialsHiddenByCleanParse(t *testing.T) {
	t.Parallel()

	const uri = "mongodb://monitor:1234/ss@127.0.0.1/admin"

	parsed, err := url.Parse(uri)
	require.NoError(t, err)
	require.Nil(t, parsed.User, "url.Parse is expected to miss the credentials here")

	assert.Equal(t, "mongodb://monitor:xxxxx@127.0.0.1/admin", MongoURI(uri))
}

// TestMongoURIWithoutCredentials checks that a URI carrying no password is left alone.
func TestMongoURIWithoutCredentials(t *testing.T) {
	t.Parallel()

	run(t, []redactTest{
		{
			name:     "no credentials",
			uri:      "mongodb://127.0.0.1:27017/admin",
			expected: "mongodb://127.0.0.1:27017/admin",
			password: "",
		}, {
			name:     "user without password",
			uri:      "mongodb://monitor@127.0.0.1:27017",
			expected: "mongodb://monitor@127.0.0.1:27017",
			password: "",
		}, {
			name:     "multiple hosts without credentials",
			uri:      "mongodb://host1:27017,host2:27017/admin?replicaSet=rs1",
			expected: "mongodb://host1:27017,host2:27017/admin?replicaSet=rs1",
			password: "",
		},
	})
}

// TestMongoURIOverRedacts pins a known and deliberate imprecision: an at sign after the authority
// pulls the match past the host. Since a password may contain the very bytes that would otherwise
// delimit it, the expression cannot tell the two apart, and mangling a credential-free URI in the
// log is the safer of the two ways to be wrong.
func TestMongoURIOverRedacts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mongodb://host:xxxxx@b", MongoURI("mongodb://host:27017/db?param=a@b"))
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

	const socket = "%2Ftmp%2Fmongodb.sock"

	for _, password := range []string{"s3cr3t", specialCharsKeptByParseError, "pa/ss", "pa ss"} {
		t.Run(password, func(t *testing.T) {
			t.Parallel()

			_, err := url.Parse("mongodb://monitor:" + password + "@" + socket + "/admin")
			require.Error(t, err)
			require.Contains(t, err.Error(), password, "url.Parse is expected to quote the URI verbatim")

			redacted := Error(err)
			assert.NotContains(t, redacted, password)
			assert.Contains(t, redacted, "mongodb://monitor:xxxxx@"+socket+"/admin")
		})
	}
}

// TestErrorRedactsEveryURIInOneMessage covers a message quoting more than one URI, which happens
// when a multi-target configuration is reported in a single error. Neither password may survive.
//
// The match is greedy, so the two URIs collapse into one and the text between them is lost. That
// costs detail in the log but discloses nothing, and narrowing the match would let a password
// containing "@" through, which is the trade this package refuses to make.
func TestErrorRedactsEveryURIInOneMessage(t *testing.T) {
	t.Parallel()

	first, second := "s3cr3t", specialCharsKeptByParseError
	err := errors.New("cannot connect to mongodb://monitor:" + first + "@host1:27017/admin " + //nolint:err113 // test fixture.
		"or to mongodb://backup:" + second + "@host2:27017/admin")

	redacted := Error(err)
	assert.NotContains(t, redacted, first)
	assert.NotContains(t, redacted, second)
	assert.Contains(t, redacted, ":xxxxx@")
	assert.Equal(t, 1, strings.Count(redacted, ":xxxxx@"), "greedy match is expected to collapse both URIs")
}

// TestErrorLeavesTruncatedURIAlone documents a residual gap rather than approving of it.
//
// "?" and "#" terminate the authority, so url.Parse reports on a URI it has already cut short: for
// the password "pa#ss" the error reads `parse "mongodb://monitor:pa": invalid port ":pa"`. No "@"
// survives for the expression to anchor on, so the part of the password before the delimiter stays
// in the message. The full password never appears, but the prefix does.
//
// Where the exporter builds the message it sidesteps this by not logging the parse error at all and
// logging the redacted URI instead. Error remains for messages that come from elsewhere -- PBM
// wraps the URI it was handed into its own text -- where dropping the error would cost the only
// diagnostic available. Widening the pattern is not an option: it could not tell "monitor:pa" from
// "host:27017".
func TestErrorLeavesTruncatedURIAlone(t *testing.T) {
	t.Parallel()

	password := "pa#ss"

	_, err := url.Parse("mongodb://monitor:" + password + "@127.0.0.1:27017/admin")
	require.Error(t, err)

	redacted := Error(err)
	assert.NotContains(t, redacted, password, "the full password must never survive")
	assert.Contains(t, redacted, "monitor:pa", "known gap: the prefix before # is still disclosed")
}

// TestErrorRedactsURIWrappedByThirdParty covers the shape percona-backup-mongodb produces: it
// quotes the URI it was given and then wraps the *url.Error, which quotes it a second time. Both
// copies carry the password, and the message reaches a default-level log on every scrape when
// --collector.pbm is on.
func TestErrorRedactsURIWrappedByThirdParty(t *testing.T) {
	t.Parallel()

	for _, password := range []string{"s3cr3t", "pa#ss", specialChars} {
		t.Run(password, func(t *testing.T) {
			t.Parallel()

			uri := "mongodb://monitor:" + password + "@%2Ftmp%2Fmongodb.sock/admin"
			_, parseErr := url.Parse(uri)
			require.Error(t, parseErr)

			// Reproduces the message built by percona-backup-mongodb.
			err := fmt.Errorf("parse mongo-uri '%s': %w", uri, parseErr)

			redacted := Error(err)
			assert.NotContains(t, redacted, password)
			assert.Contains(t, redacted, "monitor:xxxxx@")
		})
	}
}
