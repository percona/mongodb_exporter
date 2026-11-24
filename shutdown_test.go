// mongodb_exporter
// Copyright (C) 2026 Percona LLC
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

//go:build !windows

package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer safe for concurrent writes by the child
// process and reads by the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p) //nolint:wrapcheck // io.Writer passthrough
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// freeAddr returns a localhost address with a free TCP port.
func freeAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	return addr
}

// waitForListener polls the exporter's root page until it answers.
func waitForListener(t *testing.T, addr string, output *syncBuffer) {
	t.Helper()

	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/", nil)
		if err != nil {
			return false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()

		return true
	}, 15*time.Second, 50*time.Millisecond, "exporter did not start listening on %s\n%s", addr, output)
}

func TestGracefulShutdownOnSIGTERM(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("builds and runs the exporter binary")
	}

	bin := filepath.Join(t.TempDir(), "mongodb_exporter")
	out, err := exec.CommandContext(context.Background(), "go", "build", "-o", bin, ".").CombinedOutput() //nolint:gosec // bin is a path in t.TempDir()
	require.NoError(t, err, "go build: %s", out)

	addr := freeAddr(t)
	output := &syncBuffer{}
	// No MongoDB is needed: the web server starts regardless and this test only
	// exercises the process lifecycle.
	cmd := exec.CommandContext(context.Background(), bin, //nolint:gosec // bin is built by this test
		"--web.listen-address="+addr,
		"--mongodb.uri=mongodb://127.0.0.1:1",
		"--mongodb.connect-timeout-ms=100",
		"--log.level=info",
	)
	cmd.Stdout = output
	cmd.Stderr = output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	waitForListener(t, addr, output)
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		require.NoError(t, err, "exporter did not exit cleanly on SIGTERM\n%s", output)
	case <-time.After(10 * time.Second):
		t.Fatalf("exporter did not exit within 10s of SIGTERM\n%s", output)
	}

	require.Contains(t, output.String(), "shutting down",
		"exporter exited without running its shutdown path")
}
