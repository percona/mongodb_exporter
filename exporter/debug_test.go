// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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
	"io/ioutil"
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
	out, _ := ioutil.ReadAll(r)

	assert.Equal(t, want, string(out))
}
