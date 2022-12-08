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
	"io"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestGetEncryptionInfo(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.TestClient(ctx, tu.MongoDBStandAloneEncryptedPort, t)
	t.Cleanup(func() {
		err := client.Disconnect(ctx)
		assert.NoError(t, err)
	})
	logger := logrus.New()
	logger.Out = io.Discard // disable logs in tests

	ti := labelsGetterMock{}

	c := newDiagnosticDataCollector(ctx, client, logger, true, ti)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
	# HELP mongodb_security_encryption_enabled Shows that encryption is enabled
	# TYPE mongodb_security_encryption_enabled gauge
	mongodb_security_encryption_enabled{type="localKeyFile"} 1
	# HELP mongodb_version_info The server version
	# TYPE mongodb_version_info gauge
	mongodb_version_info{edition="Community",mongodb="5.0.13-11",vendor="Percona"} 1` + "\n")

	filter := []string{
		"mongodb_security_encryption_enabled",
		"mongodb_version_info",
	}

	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
