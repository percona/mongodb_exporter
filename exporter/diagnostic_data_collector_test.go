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

	client := getTestClient(t)

	c := &diagnosticDataCollector{
		ctx:    ctx,
		client: client,
	}

	expected := strings.NewReader(`
	`)
	err := testutil.CollectAndCompare(c, expected)
	assert.NoError(t, err)
}
