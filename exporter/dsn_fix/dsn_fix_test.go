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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientOptionsForDSN(t *testing.T) {
	tests := []struct {
		name             string
		dsn              string
		expectedUser     string
		expectedPassword string
	}{
		{
			name: "Escape username",
			dsn: (&url.URL{
				Scheme: "mongo",
				Host:   "localhost",
				Path:   "/db",
				User:   url.UserPassword("user+", "pass"),
			}).String(),
			expectedUser:     "user+",
			expectedPassword: "pass",
		},
		{
			name: "Escape password",
			dsn: (&url.URL{
				Scheme: "mongo",
				Host:   "localhost",
				Path:   "/db",
				User:   url.UserPassword("user", "pass+"),
			}).String(),
			expectedUser:     "user",
			expectedPassword: "pass+",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClientOptionsForDSN(tt.dsn)
			assert.Nil(t, err)
			assert.Equal(t, got.Auth.Username, tt.expectedUser)
			assert.Equal(t, got.Auth.Password, tt.expectedPassword)
		})
	}
}
