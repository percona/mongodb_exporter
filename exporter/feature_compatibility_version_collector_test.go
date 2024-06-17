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
	"strings"
	"testing"
	"time"

	"github.com/percona/mongodb_exporter/internal/tu"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFCVCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx)       //nolint:errcheck
	defer database.Drop(ctx) //nolint:errcheck

	interval := 5 * time.Second

	c := newFeatureCompatibilityCollector(ctx, client, logrus.New(), interval)
	c.now = func() time.Time {
		return time.Date(2024, 06, 14, 00, 00, 00, 00, time.UTC)
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_fcv_featureCompatibilityVersion fcv.
# TYPE mongodb_fcv_featureCompatibilityVersion untyped
mongodb_fcv_featureCompatibilityVersion{last_scrape="2024-06-14 00:00:00"} 4` +
		"\n")

	filter := []string{
		"mongodb_fcv_featureCompatibilityVersion",
	}
	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)

	// Less than 5 seconds, it should return the last scraped values.
	c.now = func() time.Time {
		return time.Date(2024, 06, 14, 00, 00, 04, 00, time.UTC)
	}

	expected = strings.NewReader(`
# HELP mongodb_fcv_featureCompatibilityVersion fcv.
# TYPE mongodb_fcv_featureCompatibilityVersion untyped
mongodb_fcv_featureCompatibilityVersion{last_scrape="2024-06-14 00:00:00"} 4` +
		"\n")
	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)

	// After more than 5 seconds there should be a new scrape.
	c.now = func() time.Time {
		return time.Date(2024, 06, 14, 00, 00, 06, 00, time.UTC)
	}
	expected = strings.NewReader(`
# HELP mongodb_fcv_featureCompatibilityVersion fcv.
# TYPE mongodb_fcv_featureCompatibilityVersion untyped
mongodb_fcv_featureCompatibilityVersion{last_scrape="2024-06-14 00:00:06"} 4` +
		"\n")

	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
