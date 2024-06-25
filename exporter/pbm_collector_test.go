package exporter

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona/mongodb_exporter/internal/tu"
)

func TestPBMCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	port, err := tu.PortForContainer("standalone-backup")
	require.NoError(t, err)
	client := tu.TestClient(ctx, port, t)
	mongoUri := "mongodb://pbm:pbm@localhost:27037"

	c := newPbmCollector(ctx, client, mongoUri, logrus.New())

	filter := []string{
		"mongodb_pbm_cluster_backup_configured",
	}
	count := testutil.CollectAndCount(c, filter...)
	assert.Equal(t, len(filter), count, "PBM metrics are missing")
}
