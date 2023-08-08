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

package exporter

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestMultiTarget(t *testing.T) {
	hostname := "127.0.0.1"
	opts := []*Opts{
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_STANDALONE_PORT", "27017"))),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"))),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, tu.GetenvDefault("TEST_MONGODB_S2_PRIMARY_PORT", "17004"))),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
		{
			URI:              fmt.Sprintf("mongodb://%s", net.JoinHostPort(hostname, "12345")),
			DirectConnect:    true,
			ConnectTimeoutMS: 1000,
		},
	}
	exporters := make([]*Exporter, len(opts))

	for i, opt := range opts {
		exporters[i] = New(opt)
	}
	log := logrus.New()
	buildServerMap(exporters, log)

	expected := []string{
		"mongodb_up 1\n",
		"mongodb_up 1\n",
		"mongodb_up 1\n",
		"mongodb_up 0\n",
	}

	// Test all targets
	for sn, opt := range opts {
		assert.HTTPBodyContains(t, http.HandlerFunc(multiTargetHandler), "GET", fmt.Sprintf("?target=%s", opt.URI), nil, expected[sn])
	}
}
