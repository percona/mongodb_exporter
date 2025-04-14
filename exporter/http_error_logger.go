// mongodb_exporter
// Copyright (C) 2025 Percona LLC
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
	"fmt"
	"log/slog"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// httpErrorLogger is a wrapper around slog.Logger that can log promhttp errors (by implementing a Println method).
type httpErrorLogger struct {
	logger *slog.Logger
}

func newHTTPErrorLogger(logger *slog.Logger) *httpErrorLogger {
	return &httpErrorLogger{logger: logger}
}

// Println implements the Println method for httpErrorHandler.
func (h *httpErrorLogger) Println(v ...any) {
	// promhttp calls the Println() method as follows:
	// logger.Println(message, err) i.e., v[0] is the message (a string) and v[1] is the error (which might be prometheus.MultiError)
	if len(v) == 2 {
		msg, mok := v[0].(string)
		err, eok := v[1].(error)
		if mok && eok {
			multiErr := prometheus.MultiError{}
			if errors.As(err, &multiErr) {
				errCount := len(multiErr)
				for i, err := range multiErr {
					h.logger.Error(msg, "error", err, "total_errors", errCount, "error_no", i)
				}
			}
		}
	}
	// fallback
	h.logger.Error(fmt.Sprint(v...))
}
