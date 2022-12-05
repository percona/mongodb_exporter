package exporter

import (
	"context"
	"github.com/percona/mongodb_exporter/internal/tu"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestGetEncryptionInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := tu.StandaloneEncryptedClient(ctx, t)
	defer client.Disconnect(ctx)

	logger := logrus.New()
	logger.Out = ioutil.Discard // diable logs in tests

	ti := labelsGetterMock{}

	c := newDiagnosticDataCollector(ctx, client, logger, true, ti)

	// The last \n at the end of this string is important
	expected := strings.NewReader(`
	# HELP mongodb_security_encryption_enabled Shows that encryption is enabled
	# TYPE mongodb_security_encryption_enabled gauge
	mongodb_security_encryption_enabled 1` + "\n")
	// Filter metrics for 2 reasons:
	// 1. The result is huge
	// 2. We need to check against know values. Don't use metrics that return counters like uptime
	//    or counters like the number of transactions because they won't return a known value to compare
	filter := []string{
		"mongodb_security_encryption_enabled",
	}

	err := testutil.CollectAndCompare(c, expected, filter...)
	assert.NoError(t, err)
}
