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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestFCVCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.DefaultTestClient(ctx, t)

	database := client.Database("testdb")
	database.Drop(ctx)       //nolint:errcheck
	defer database.Drop(ctx) //nolint:errcheck

	c := newFeatureCompatibilityCollector(ctx, client, promslog.New(&promslog.Config{}))

	sversion, _ := getMongoDBVersionInfo(t, "mongo-1-1")

	v, err := version.NewVersion(sversion)
	require.NoError(t, err)
	var mversion string

	mmv := fmt.Sprintf("%d.%d", v.Segments()[0], v.Segments()[1])
	switch {
	case mmv == "5.0":
		mversion = "4.4"
	case mmv == "4.4":
		mversion = "4.2"
	case mmv == "6.0":
		mversion = "5.0"
	case mmv == "7.0":
		mversion = "6.0"
	case mmv == "8.0":
		mversion = "7.0"
	default:
		mversion = mmv
	}

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
# HELP mongodb_fcv_feature_compatibility_version Feature compatibility version
# TYPE mongodb_fcv_feature_compatibility_version gauge
mongodb_fcv_feature_compatibility_version{version="` + mversion + `"} ` + mversion +
		"\n")

	filter := []string{
		"mongodb_fcv_feature_compatibility_version",
	}
	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)

	expected = strings.NewReader(`
# HELP mongodb_fcv_feature_compatibility_version Feature compatibility version
# TYPE mongodb_fcv_feature_compatibility_version gauge
mongodb_fcv_feature_compatibility_version{version="` + mversion + `"} ` + mversion +
		"\n")
	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)

	expected = strings.NewReader(`
# HELP mongodb_fcv_feature_compatibility_version Feature compatibility version
# TYPE mongodb_fcv_feature_compatibility_version gauge
mongodb_fcv_feature_compatibility_version{version="` + mversion + `"} ` + mversion +
		"\n")

	err = testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
