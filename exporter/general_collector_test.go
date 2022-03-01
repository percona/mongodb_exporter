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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestGeneralCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	base := newBaseCollector(client, logrus.New())
	c := newGeneralCollector(ctx, base)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
	# HELP mongodb_up Whether MongoDB is up.
	# TYPE mongodb_up gauge
	mongodb_up 1` + "\n")
	err := testutil.CollectAndCompare(c, expected)
	require.NoError(t, err)

	assert.NoError(t, client.Disconnect(ctx))

	expected = strings.NewReader(`
	# HELP mongodb_up Whether MongoDB is up.
	# TYPE mongodb_up gauge
	mongodb_up 0` + "\n")
	err = testutil.CollectAndCompare(c, expected)
	require.NoError(t, err)
}
