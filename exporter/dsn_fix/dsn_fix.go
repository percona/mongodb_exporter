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
		_clientOptions := options.Credential{Username: username, Password: password, PasswordSet: true}

		// Initialize fields other than username and password to values parsed by ApplyURI()
		if clientOptions.Auth != nil {
			_clientOptions.AuthMechanism = clientOptions.Auth.AuthMechanism
			_clientOptions.AuthMechanismProperties = clientOptions.Auth.AuthMechanismProperties
			_clientOptions.AuthSource = clientOptions.Auth.AuthSource
			_clientOptions.PasswordSet = clientOptions.Auth.PasswordSet
		} else if parsedDsn.Path != "/" && parsedDsn.Path != "" {
			// When clientOptions.Auth nil, salvage AuthSource from parsedDsn
			// This can happen when an invalid username or password are passed to ApplyURI()
			_clientOptions.AuthSource = parsedDsn.Path
		}
		clientOptions = clientOptions.SetAuth(_clientOptions)
	}

	return clientOptions, nil
}
