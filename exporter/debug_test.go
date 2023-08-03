// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDebug(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	olderr := os.Stderr
	r, w, _ := os.Pipe()

	os.Stderr = w
	defer func() {
		os.Stderr = olderr
		logrus.SetLevel(logrus.ErrorLevel)
	}()

	log.Out = w

	m := bson.M{
		"f1": 1,
		"f2": "v2",
		"f3": bson.M{
			"f4": 4,
		},
	}
	want := `{
  "f1": 1,
  "f2": "v2",
  "f3": {
    "f4": 4
  }
}` + "\n"

	debugResult(log, m)
	assert.NoError(t, w.Close())
	out, _ := io.ReadAll(r)

	assert.Equal(t, want, string(out))
}
