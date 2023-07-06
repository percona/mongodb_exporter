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
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func debugResult(log *logrus.Logger, m interface{}) {
	if !log.IsLevelEnabled(logrus.DebugLevel) {
		return
	}

	debugStr, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Errorf("cannot marshal struct for debug: %s", err)
		return
	}

	// don't use logrus because:
	// 1. It will escape new lines and " making it harder to read and to use
	// 2. It will add timestamp
	// 3. This way is easier to copy/paste to put the info in a ticket
	fmt.Fprintln(os.Stderr, string(debugStr))
}
