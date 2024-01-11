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
