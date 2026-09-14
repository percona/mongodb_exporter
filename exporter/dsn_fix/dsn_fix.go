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

package dsn_fix

import (
	"net/url"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ClientOptionsForDSN applies URI to Client.
func ClientOptionsForDSN(dsn string) (*options.ClientOptions, error) {
	clientOptions := options.Client().ApplyURI(dsn).SetBSONOptions(&options.BSONOptions{
		// Preserve the v1 driver's default for interface/map decoding. The v2
		// driver defaults nested documents to bson.D instead of bson.M.
		DefaultDocumentM: true,
	})
	if e := clientOptions.Validate(); e != nil {
		return nil, e
	}

	// Workaround for PMM-9320
	// if username or password is set, need to replace it with correctly parsed credentials.
	parsedDsn, err := url.Parse(dsn)
	if err != nil {
		// for non-URI, do nothing (PMM-10265)
		return clientOptions, nil
	}
	username := parsedDsn.User.Username()
	password, _ := parsedDsn.User.Password()
	if username != "" || password != "" {
		clientOptions.Auth.Username = username
		clientOptions.Auth.Password = password
	}

	return clientOptions, nil
}
