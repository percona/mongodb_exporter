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

// Package redact hides the credentials embedded in MongoDB connection URIs before they are logged.
package redact

import (
	"net/url"
	"regexp"
)

// credentialsRE matches the userinfo of a MongoDB connection URI that carries a password. It is
// used when url.Parse is of no help: either the URI is malformed, or it is quoted inside a longer
// string such as an error message.
var credentialsRE = regexp.MustCompile(`(mongodb(?:\+srv)?://)([^\s/?#]*):([^\s/?#]*)@`)

// replacement keeps the scheme and the user name and drops the password. It matches what
// (*url.URL).Redacted does.
const replacement = "${1}${2}:xxxxx@"

// MongoURI returns uri with its password replaced by a placeholder, so that it is safe to log.
func MongoURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err == nil && parsed.User != nil {
		return parsed.Redacted()
	}

	return credentialsRE.ReplaceAllString(uri, replacement)
}

// Error returns the message of err with the password of any MongoDB URI it quotes replaced by a
// placeholder. url.Parse and the MongoDB driver embed the offending URI in their error messages,
// so logging the error alone is enough to leak the credentials.
func Error(err error) string {
	if err == nil {
		return ""
	}

	return credentialsRE.ReplaceAllString(err.Error(), replacement)
}
