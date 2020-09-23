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
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/percona/exporter_shared/helpers"
	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestServerStatusDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)
	ti := labelsGetterMock{}

	c := &serverStatusCollector{
		client:       client,
		logger:       logrus.New(),
		topologyInfo: ti,
	}

	metrics := helpers.CollectMetrics(c)
	actualMetrics := zeroMetrics(helpers.ReadMetrics(metrics))
	actualLines := helpers.Format(helpers.WriteMetrics(actualMetrics))

	samplesFile := "testdata/all_server_status_data.json"
	if isTrue, _ := strconv.ParseBool(os.Getenv("UPDATE_SAMPLES")); isTrue {
		assert.NoError(t, writeJSON(samplesFile, actualLines))
	}

	var wantLines []string
	assert.NoError(t, readJSON(samplesFile, &wantLines))

	assert.Equal(t, wantLines, actualLines)
}
