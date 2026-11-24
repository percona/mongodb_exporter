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
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// encodingErrorMsg is the message promhttp logs when writing a metric family to the response fails.
const encodingErrorMsg = "error encoding and sending metric family"

// httpErrorLogger is a wrapper around slog.Logger that can log promhttp errors (by implementing a Println method).
// A new httpErrorLogger must be created for every request.
type httpErrorLogger struct {
	logger *slog.Logger

	mu                 sync.Mutex
	encodingErrorCount int
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
			if strings.HasPrefix(msg, encodingErrorMsg) {
				h.logEncodingError(err)

				return
			}

			multiErr := prometheus.MultiError{}
			if errors.As(err, &multiErr) {
				errCount := len(multiErr)
				for i, err := range multiErr {
					h.logger.Error(msg, "error", err, "total_errors", errCount, "error_no", i)
				}

				return
			}
		}
	}
	// fallback
	h.logger.Error(fmt.Sprint(v...))
}

// logEncodingError logs only the first encoding error of a request.
// When the client closes the connection (for example, after a scrape or probe timeout),
// promhttp keeps trying to write every remaining metric family and each write fails
// with the same error, so logging all of them floods the log.
func (h *httpErrorLogger) logEncodingError(err error) {
	h.mu.Lock()
	h.encodingErrorCount++
	first := h.encodingErrorCount == 1
	h.mu.Unlock()

	if first {
		h.logger.Warn("Error sending metrics to the client, the client might have closed the connection "+
			"(check the scrape or probe timeout)", "error", err)
	}
}

// logSuppressedErrors logs how many encoding errors were not logged. Call it after the response has been written.
func (h *httpErrorLogger) logSuppressedErrors() {
	h.mu.Lock()
	suppressed := h.encodingErrorCount - 1
	h.mu.Unlock()

	if suppressed > 0 {
		h.logger.Debug("Suppressed repeated errors sending metrics to the client", "count", suppressed)
	}
}
