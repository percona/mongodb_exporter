// mongodb_exporter
// Copyright (C) 2017 Percona LLC
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

// Package tu has Test Util functions
package tu

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// MongosPort MongoDB mongos Port.
	MongosPort = "17000"
	// MongoDBS1PrimaryPort MongoDB Shard 1 Primary Port.
	MongoDBS1PrimaryPort = "17001"
	// MongoDBS1Secondary1Port MongoDB Shard 1 Secondary 1 Port.
	MongoDBS1Secondary1Port = "17002"
	// MongoDBS1Secondary2Port MongoDB Shard 1 Secondary 2 Port.
	MongoDBS1Secondary2Port = "17003"
	// MongoDBStandAlonePort MongoDB stand alone instance Port.
	MongoDBStandAlonePort = "27017"
	// MongoDBConfigServer1Port MongoDB config server primary Port.
	MongoDBConfigServer1Port = "17009"
	// MongoDBStandAloneEncryptedPort MongoDB standalone encrypted instance Port.
	MongoDBStandAloneEncryptedPort = "27027"
)

// GetenvDefault gets a variable from the environment and returns its value or the
// spacified default if the variable is empty.
func GetenvDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}

// DefaultTestClient returns the default MongoDB connection used for tests. It is a direct
// connection to the primary server of replicaset 1.
func DefaultTestClient(ctx context.Context, t *testing.T) *mongo.Client {
	port, err := PortForContainer("mongo-1-1")
	require.NoError(t, err)

	return TestClient(ctx, port, t)
}

// DefaultTestClientMongoS returns the mongos MongoDB connection used for tests. It is a direct
// connection to the mongos server.
func DefaultTestClientMongoS(ctx context.Context, t *testing.T) *mongo.Client {
	t.Helper()

	port, err := PortForContainer("mongos")
	require.NoError(t, err)

	return TestClient(ctx, port, t)
}

// GetImageNameForContainer returns image name and version of a running test container.
func GetImageNameForContainer(containerName string) (string, string, error) {
	di, err := InspectContainer(containerName)
	if err != nil {
		return "", "", errors.Wrapf(err, "cannot get error for container %q", "mongo-1-1")
	}

	if len(di) == 0 {
		return "", "", errors.Wrapf(err, "cannot get error for container %q (empty array)", "mongo-1-1")
	}

	split := strings.Split(di[0].Config.Image, ":")

	const numOfImageNameParts = 2
	if len(split) != numOfImageNameParts {
		return "", "", errors.New(fmt.Sprintf("image name is not correct: %s", di[0].Config.Image))
	}

	imageBaseName, version := split[0], split[1]

	for _, s := range di[0].Config.Env {
		if strings.HasPrefix(s, "MONGO_VERSION=") {
			version = strings.ReplaceAll(s, "MONGO_VERSION=", "")

			break
		}
		if strings.HasPrefix(s, "PSMDB_VERSION=") {
			version = strings.ReplaceAll(s, "PSMDB_VERSION=", "")

			break
		}
	}

	return imageBaseName, version, nil
}

// TestClient returns a new MongoDB connection to the specified server port.
func TestClient(ctx context.Context, port string, t *testing.T) *mongo.Client {
	if port == "" {
		port = MongoDBS1PrimaryPort
	}

	hostname := "127.0.0.1"
	direct := true
	to := time.Second
	co := &options.ClientOptions{
		ConnectTimeout: &to,
		Hosts:          []string{net.JoinHostPort(hostname, port)},
		Direct:         &direct,
	}

	client, err := mongo.Connect(ctx, co)
	require.NoError(t, err)

	t.Cleanup(func() {
		// In some tests we manually disconnect the client so, don't check
		// for errors, the client might be already disconnected.
		client.Disconnect(ctx) //nolint:errcheck
	})

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	return client
}

// LoadJSON loads a file and returns the result of unmarshaling it into a bson.M structure.
func LoadJSON(filename string) (bson.M, error) {
	buf, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}

	var m bson.M
	err = json.Unmarshal(buf, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func InspectContainer(name string) (DockerInspectOutput, error) {
	var di DockerInspectOutput

	out, err := exec.Command("docker", "inspect", name).Output() //nolint:gosec
	if err != nil {
		return di, errors.Wrap(err, "cannot inspect docker container")
	}

	if err := json.Unmarshal(out, &di); err != nil {
		return di, errors.Wrap(err, "cannot inspect docker container")
	}

	return di, nil
}

func PortForContainer(name string) (string, error) {
	di, err := InspectContainer(name)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get error for container %q", name)
	}

	if len(di) == 0 {
		return "", errors.Wrapf(err, "cannot get error for container %q (empty array)", name)
	}

	ports := di[0].NetworkSettings.Ports["27017/tcp"]
	if len(ports) == 0 {
		return "", errors.Wrapf(err, "cannot get error for container %q (empty ports list)", name)
	}

	return ports[0].HostPort, nil
}

// IPForContainer returns the IP address of a running container.
func IPForContainer(name string) (string, error) {
	di, err := InspectContainer(name)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get error for container %q", name)
	}

	if len(di) == 0 {
		return "", errors.Wrapf(err, "cannot get error for container %q (empty array)", name)
	}

	return di[0].NetworkSettings.Networks.MongodbExporterDefault.IPAddress, nil
}

// SetupFakeResolver sets up Fake DNS server to resolve SRV records.
func SetupFakeResolver() *mockdns.Server {
	p1, err1 := strconv.ParseInt(GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"), 10, 64)
	p2, err2 := strconv.ParseInt(GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "17002"), 10, 64)
	p3, err3 := strconv.ParseInt(GetenvDefault("TEST_MONGODB_S1_SECONDARY2_PORT", "17003"), 10, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		panic("Invalid ports")
	}

	testZone := map[string]mockdns.Zone{
		"_mongodb._tcp.server.example.com.": {
			SRV: []net.SRV{
				{
					Target: "mongo1.example.com.",
					Port:   uint16(p1),
				},
				{
					Target: "mongo2.example.com.",
					Port:   uint16(p2),
				},
				{
					Target: "mongo3.example.com.",
					Port:   uint16(p3),
				},
			},
		},
		"server.example.com.": {
			TXT: []string{"authSource=admin"},
			A:   []string{"1.2.3.4"},
		},
		"mongo1.example.com.": {
			A: []string{"127.0.0.1"},
		},
		"mongo2.example.com.": {
			A: []string{"127.0.0.1"},
		},
		"mongo3.example.com.": {
			A: []string{"127.0.0.1"},
		},
		"unexistent.com.": {
			A: []string{"127.0.0.1"},
		},
	}

	srv, _ := mockdns.NewServer(testZone, true)
	srv.PatchNet(net.DefaultResolver)

	return srv
}
