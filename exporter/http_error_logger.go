package exporter

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
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
