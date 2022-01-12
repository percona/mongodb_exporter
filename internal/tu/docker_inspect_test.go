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

package tu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInspectContainer(t *testing.T) {
	tests := []struct {
		containerName string
		wantPort      string
	}{
		{
			containerName: "mongos",
			wantPort:      "17000",
		},
		{
			containerName: "standalone",
			wantPort:      "27017",
		},
	}

	for _, tc := range tests {
		di, err := InspectContainer(tc.containerName)
		assert.NoError(t, err)

		ns := di[0].NetworkSettings.Ports["27017/tcp"][0].HostPort
		assert.Equal(t, ns, tc.wantPort)
	}
}
