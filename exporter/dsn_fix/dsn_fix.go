// mongodb_exporter
// Copyright (C) 2017 Percona LLC
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

package dsn_fix

import (
	"net/url"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// ClientOptionsForDSN applies URI to Client.
func ClientOptionsForDSN(dsn string) (*options.ClientOptions, error) {
	clientOptions := options.Client().ApplyURI(dsn)
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
		// set this flag to connect to arbiter when there authentication is enabled
		b := true
		clientOptions.AuthenticateToAnything = &b
	}

	return clientOptions, nil
}
