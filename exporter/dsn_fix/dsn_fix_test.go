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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientOptionsForDSN(t *testing.T) {
	tests := []struct {
		name             string
		error            string
		dsn              string
		expectedUser     string
		expectedPassword string
	}{
		{
			name: "Escape username",
			dsn: (&url.URL{
				Scheme: "mongodb",
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
				Scheme: "mongodb",
				Host:   "localhost",
				Path:   "/db",
				User:   url.UserPassword("user", "pass+"),
			}).String(),
			expectedUser:     "user",
			expectedPassword: "pass+",
		},
		{
			name: "Invalid URI",
			dsn: (&url.URL{
				Scheme: "<invalid>",
				Host:   "localhost",
				Path:   "/db",
				User:   url.UserPassword("user", "pass+"),
			}).String(),
			error:            "error parsing uri: scheme must be \"mongodb\" or \"mongodb+srv\"",
			expectedUser:     "user",
			expectedPassword: "pass+",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClientOptionsForDSN(tt.dsn)
			if tt.error != "" {
				assert.Equal(t, err.Error(), tt.error)
			} else {
				assert.Empty(t, err)
				assert.Equal(t, got.Auth.Username, tt.expectedUser)
				assert.Equal(t, got.Auth.Password, tt.expectedPassword)
			}
		})
	}
}
