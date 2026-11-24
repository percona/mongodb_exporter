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
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTest1 = errors.New("err1")
	errTest2 = errors.New("err2")
)

// brokenConnWriter behaves like a response writer whose client has closed the connection.
type brokenConnWriter struct {
	header http.Header
}

func (w *brokenConnWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}

	return w.header
}

func (w *brokenConnWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write tcp 127.0.0.1:9216->127.0.0.1:50000: %w", syscall.ECONNRESET)
}

func (w *brokenConnWriter) WriteHeader(int) {}

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestHTTPErrorLoggerClosedConnection checks that a client closing the connection
// produces one log line per request instead of one per metric family (issue #632).
func TestHTTPErrorLoggerClosedConnection(t *testing.T) {
	t.Parallel()

	const families = 500

	registry := prometheus.NewRegistry()
	for i := range families {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: fmt.Sprintf("test_metric_%d", i), Help: "test"})
		registry.MustRegister(g)
	}

	var buf bytes.Buffer
	errorLog := newHTTPErrorLogger(newTestLogger(&buf))
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
		ErrorLog:      errorLog,
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	h.ServeHTTP(&brokenConnWriter{}, req)
	errorLog.logSuppressedErrors()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2, buf.String())
	assert.Contains(t, lines[0], "level=WARN")
	assert.Contains(t, lines[0], "connection reset by peer")
	assert.Contains(t, lines[1], "level=DEBUG")
	assert.Contains(t, lines[1], fmt.Sprintf("count=%d", families-1))
}

func TestHTTPErrorLoggerGatherErrors(t *testing.T) {
	t.Parallel()

	t.Run("multi error is logged once per error", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		errorLog := newHTTPErrorLogger(newTestLogger(&buf))
		errorLog.Println("error gathering metrics:", prometheus.MultiError{errTest1, errTest2})
		errorLog.logSuppressedErrors()

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 2, buf.String())
		assert.Contains(t, lines[0], "error=err1")
		assert.Contains(t, lines[1], "error=err2")
	})

	t.Run("single error", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		errorLog := newHTTPErrorLogger(newTestLogger(&buf))
		errorLog.Println("error gathering metrics:", errTest1)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 1, buf.String())
		assert.Contains(t, lines[0], "level=ERROR")
		assert.Contains(t, lines[0], "err1")
	})

	t.Run("every gather error is logged", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		errorLog := newHTTPErrorLogger(newTestLogger(&buf))
		errorLog.Println("error gathering metrics:", errTest1)
		errorLog.Println("error gathering metrics:", errTest2)
		errorLog.logSuppressedErrors()

		assert.Equal(t, 2, strings.Count(buf.String(), "level=ERROR"), buf.String())
		assert.NotContains(t, buf.String(), "Suppressed")
	})
}
