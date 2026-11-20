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
	"errors"
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

	localhostIP = "127.0.0.1"
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
	t.Helper()

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

// GetImageNameForContainer returns image name and version of a test container, running or not.
// TestGetEncryptionInfo needs the latter: upstream MongoDB cannot start standalone-encrypted, and
// the test reads its image to skip.
func GetImageNameForContainer(containerName string) (string, string, error) {
	di, err := InspectContainer(containerName)
	if err != nil {
		return "", "", err
	}

	split := strings.Split(di[0].Config.Image, ":")

	const numOfImageNameParts = 2
	if len(split) != numOfImageNameParts {
		return "", "", fmt.Errorf("%w: %s", errMalformedImageName, di[0].Config.Image)
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

	hostname := localhostIP
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
		return nil, fmt.Errorf("cannot read %q: %w", filename, err)
	}

	var m bson.M
	err = json.Unmarshal(buf, &m)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal %q: %w", filename, err)
	}

	return m, nil
}

// InspectContainer returns the docker inspect output for the container name.
func InspectContainer(name string) (DockerInspectOutput, error) {
	var di DockerInspectOutput

	out, err := exec.Command("docker", "inspect", name).Output() //nolint:gosec,noctx
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return di, fmt.Errorf("cannot inspect docker container %q: %w", name, err)
		}

		// docker exits 1 and prints [] both for a missing container and for an unreachable daemon.
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if strings.Contains(strings.ToLower(stderr), "no such object") {
			return di, fmt.Errorf("%w: %q, %s", errContainerMissing, name, startTestClusterHint)
		}

		return di, fmt.Errorf("cannot inspect docker container %q: %s: %w", name, stderr, err)
	}

	err = json.Unmarshal(out, &di)
	if err != nil {
		return di, fmt.Errorf("cannot inspect docker container %q: %w", name, err)
	}

	return di, nil
}

const startTestClusterHint = "start the test cluster with `make test-cluster`"

var (
	errContainerMissing   = errors.New("container does not exist")
	errContainerStopped   = errors.New("container is not running")
	errNoHostPort         = errors.New("container publishes no host port for 27017/tcp")
	errNoContainerAddress = errors.New("container has no address on the exporter's docker network")
	errMalformedImageName = errors.New("image name is not correct")
)

// runningContainer inspects name and fails unless the container is up.
//
// docker inspect succeeds for a container that exists but has stopped, and its address and port
// fields are empty by then. Without this check a caller gets an empty string back and fails much
// later on, against an address that was never there, rather than being told which container of
// the test cluster is not running.
//
// State.Running does not do for the check: docker sets it for a paused or restarting container
// too, and a paused one keeps its address and ports while nothing in it answers.
func runningContainer(name string) (DockerInspectOutput, error) {
	di, err := InspectContainer(name)
	if err != nil {
		return nil, err
	}

	state := di[0].State
	if state.Status == "running" {
		return di, nil
	}

	// make test-cluster starts a created or exited container, and fails on a paused one.
	var hint string

	switch state.Status {
	case "created", "exited":
		hint = ", " + startTestClusterHint
	case "paused":
		hint = fmt.Sprintf(", unpause it with `docker unpause %s`", name)
	}

	return nil, fmt.Errorf("%w: %q (state: %s, exit code: %d)%s",
		errContainerStopped, name, state.Status, state.ExitCode, hint)
}

// PortForContainer returns the host port a running container publishes for 27017/tcp.
func PortForContainer(name string) (string, error) {
	di, err := runningContainer(name)
	if err != nil {
		return "", err
	}

	ports := di[0].NetworkSettings.Ports["27017/tcp"]
	if len(ports) == 0 {
		return "", fmt.Errorf("%w: %q", errNoHostPort, name)
	}

	return ports[0].HostPort, nil
}

// IPForContainer returns the IP address of a running container.
func IPForContainer(name string) (string, error) {
	di, err := runningContainer(name)
	if err != nil {
		return "", err
	}

	address := di[0].NetworkSettings.Networks.MongodbExporterDefault.IPAddress
	if address == "" {
		return "", fmt.Errorf("%w: %q", errNoContainerAddress, name)
	}

	return address, nil
}

// SetupFakeResolver sets up Fake DNS server to resolve SRV records.
func SetupFakeResolver() *mockdns.Server {
	p1, err1 := strconv.ParseUint(GetenvDefault("TEST_MONGODB_S1_PRIMARY_PORT", "17001"), 10, 16)
	p2, err2 := strconv.ParseUint(GetenvDefault("TEST_MONGODB_S1_SECONDARY1_PORT", "17002"), 10, 16)
	p3, err3 := strconv.ParseUint(GetenvDefault("TEST_MONGODB_S1_SECONDARY2_PORT", "17003"), 10, 16)

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
			A: []string{localhostIP},
		},
		"mongo2.example.com.": {
			A: []string{localhostIP},
		},
		"mongo3.example.com.": {
			A: []string{localhostIP},
		},
		"unexistent.com.": {
			A: []string{localhostIP},
		},
	}

	srv, _ := mockdns.NewServer(testZone, true)
	srv.PatchNet(net.DefaultResolver)

	return srv
}
