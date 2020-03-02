package exporter

import (
	"testing"

	"gotest.tools/assert"
)

func TestMakeRawMetricName(t *testing.T) {
	for name, fqName := range map[string]string{
		"dd.serverStatus.opcountersRepl.delete": "dd_server_status_opcounters_repl_delete",
	} {
		assert.Equal(t, fqName, makeRawMetricName(name))
	}
}
