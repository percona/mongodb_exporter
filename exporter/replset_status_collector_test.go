// mnogo_exporter
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
	"github.com/stretchr/testify/assert"

	"github.com/Percona-Lab/mnogo_exporter/internal/tu"
)

func TestReplsetStatusCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	c := &replSetGetStatusCollector{
		ctx:    ctx,
		client: client,
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
                # HELP mongodb_myState myState
                # TYPE mongodb_myState untyped
                mongodb_myState 1
                # HELP mongodb_ok ok
                # TYPE mongodb_ok untyped
                mongodb_ok 1
                # HELP mongodb_optimes_appliedOpTime_t optimes.appliedOpTime.
                # TYPE mongodb_optimes_appliedOpTime_t untyped
                mongodb_optimes_appliedOpTime_t 1
                # HELP mongodb_optimes_durableOpTime_t optimes.durableOpTime.
                # TYPE mongodb_optimes_durableOpTime_t untyped
                mongodb_optimes_durableOpTime_t 1` + "\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_myState",
		"mongodb_ok",
		"mongodb_optimes_appliedOpTime_t",
		"mongodb_optimes_durableOpTime_t",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}

func TestReplsetStatusCollectorNoSharding(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.TestClient(ctx, tu.MongoDBStandAlonePort, t)

	c := &replSetGetStatusCollector{
		ctx:    ctx,
		client: client,
	}

	expected := strings.NewReader(``)
	err := testutil.CollectAndCompare(c, expected)
	assert.NoError(t, err)
}
