package exporter

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getTestClient(ctx context.Context, t *testing.T) *mongo.Client {
	hostname := "127.0.0.1"
	port := os.Getenv("TEST_MONGODB_S1_PRIMARY_PORT") // standalone instance
	direct := true
	co := &options.ClientOptions{
		Auth: &options.Credential{
			Username:    os.Getenv("TEST_MONGODB_ADMIN_USERNAME"),
			Password:    os.Getenv("TEST_MONGODB_ADMIN_PASSWORD"),
			PasswordSet: true,
		},
		Hosts:  []string{net.JoinHostPort(hostname, port)},
		Direct: &direct,
	}

	client, err := mongo.Connect(ctx, co)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Disconnect(ctx)
		assert.NoError(t, err)
	})

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	return client
}

func TestConnect(t *testing.T) {
	hostname := "127.0.0.1"
	username := os.Getenv("TEST_MONGODB_ADMIN_USERNAME")
	password := os.Getenv("TEST_MONGODB_ADMIN_PASSWORD")
	ctx := context.Background()

	ports := map[string]string{
		"standalone":          os.Getenv("TEST_MONGODB_STANDALONE_PORT"),
		"shard-1 primary":     os.Getenv("TEST_MONGODB_S1_PRIMARY_PORT"),
		"shard-1 secondary-1": os.Getenv("TEST_MONGODB_S1_SECONDARY1_PORT"),
		"shard-1 secondary-2": os.Getenv("TEST_MONGODB_S1_SECONDARY2_PORT"),
		"shard-2 primary":     os.Getenv("TEST_MONGODB_S2_PRIMARY_PORT"),
		"shard-2 secondary-1": os.Getenv("TEST_MONGODB_S2_SECONDARY1_PORT"),
		"shard-2 secondary-2": os.Getenv("TEST_MONGODB_S2_SECONDARY2_PORT"),
		"config server 1":     os.Getenv("TEST_MONGODB_CONFIGSVR1_PORT"),
		"mongos":              os.Getenv("TEST_MONGODB_MONGOS_PORT"),
	}

	t.Run("Connect without SSL", func(t *testing.T) {
		for name, port := range ports {
			dsn := fmt.Sprintf("mongodb://%s:%s@%s:%s/admin", username, password, hostname, port)
			client, err := connect(ctx, dsn)
			assert.NoError(t, err, name)
			err = client.Disconnect(ctx)
			assert.NoError(t, err, name)
		}
	})

	t.Run("Connect with SSL", func(t *testing.T) {
		sslOpts := "ssl=true&tlsInsecure=true&tlsCertificateKeyFile=../docker/test/ssl/client.pem"
		for name, port := range ports {
			dsn := fmt.Sprintf("mongodb://%s:%s@%s:%s/admin?%s", username, password, hostname, port, sslOpts)
			client, err := connect(ctx, dsn)
			assert.NoError(t, err, name)
			err = client.Disconnect(ctx)
			assert.NoError(t, err, name)
		}
	})
}
