package exporter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestDiagnosticDataCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := getTestClient(ctx, t)

	c := &diagnosticDataCollector{
		ctx:    ctx,
		client: client,
	}

	filter := []string{"some_invalid_metric"}
	expected := strings.NewReader(`
	`)
	err := testutil.CollectAndCompare(c, expected, filter...)
	// TODO: When metric renaming is in place, we shouldn't receive an error
	// now, there are metrics with duplicated names and labels because we are not handling
	// maps and arrays in the command response
	assert.Error(t, err, "gathering metrics failed")
}
