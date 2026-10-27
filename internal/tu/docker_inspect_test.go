// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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

package tu

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		containerName string
		wantPort      string
	}{
		{
			containerName: "mongos",
			wantPort:      "17000",
		},
		{
			containerName: "standalone",
			wantPort:      "27017",
		},
	}

	for _, tc := range tests {
		di, err := InspectContainer(tc.containerName)
		require.NoError(t, err)

		ns := di[0].NetworkSettings.Ports["27017/tcp"][0].HostPort
		assert.Equal(t, tc.wantPort, ns)
	}
}

// A container that exists but is not running answers docker inspect with an empty address and
// no published ports, and both helpers used to hand that straight back as an empty string with
// a nil error: errors.Wrapf returns nil when the error it wraps is nil, so every guard below
// the inspect reported success. The caller then failed much later against a host that had never
// existed, saying nothing about the container that was down.
//
// A paused container fails differently: docker reports it as Running, with its address and ports
// in place, and the caller hangs on a connection nothing answers.
func TestContainerHelpersFailWhenContainerIsNotRunning(t *testing.T) {
	t.Parallel()

	// From an image the test cluster has already pulled.
	di, err := InspectContainer("standalone")
	require.NoError(t, err)
	require.NotEmpty(t, di)

	image := di[0].Config.Image

	setups := map[string]func(t *testing.T, name string){
		"created": func(t *testing.T, name string) {
			t.Helper()
			docker(t, "create", "--name", name, image)
		},
		"paused": func(t *testing.T, name string) {
			t.Helper()
			docker(t, "run", "--detach", "--name", name, "--entrypoint", "sleep", image, "infinity")
			docker(t, "pause", name)
		},
	}

	helpers := map[string]func(string) (string, error){
		"IPForContainer":   IPForContainer,
		"PortForContainer": PortForContainer,
	}

	for state, setup := range setups {
		t.Run(state, func(t *testing.T) {
			t.Parallel()

			name := "mongodb-exporter-" + state + "-container-test"

			_ = exec.CommandContext(t.Context(), "docker", "rm", "-f", name).Run() //nolint:gosec

			t.Cleanup(func() {
				// Not t.Context(): it is cancelled before cleanups run.
				_ = exec.CommandContext(context.Background(), "docker", "rm", "-f", name).Run() //nolint:gosec
			})

			setup(t, name)

			for helper, fn := range helpers {
				t.Run(helper, func(t *testing.T) {
					t.Parallel()

					got, err := fn(name)

					require.ErrorIs(t, err, errContainerStopped)
					assert.Empty(t, got)
					assert.Contains(t, err.Error(), name, "the error does not name the container that is down")
				})
			}

			// The image is still known, see GetImageNameForContainer.
			t.Run("GetImageNameForContainer", func(t *testing.T) {
				t.Parallel()

				gotImage, version, err := GetImageNameForContainer(name)

				require.NoError(t, err)
				assert.Equal(t, strings.Split(image, ":")[0], gotImage)
				assert.NotEmpty(t, version)
			})
		})
	}
}

// docker runs a docker command and fails the test if it does.
func docker(t *testing.T, args ...string) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "docker", args...).CombinedOutput() //nolint:gosec
	require.NoError(t, err, string(out))
}

// docker inspect exits 1 for a container that does not exist, so the helpers never saw an empty
// result and the hint to start the test cluster was unreachable.
func TestContainerHelpersFailWhenContainerIsMissing(t *testing.T) {
	t.Parallel()

	helpers := map[string]func(string) (string, error){
		"IPForContainer":   IPForContainer,
		"PortForContainer": PortForContainer,
		"GetImageNameForContainer": func(name string) (string, error) {
			image, _, err := GetImageNameForContainer(name)

			return image, err
		},
	}

	for helper, fn := range helpers {
		t.Run(helper, func(t *testing.T) {
			t.Parallel()

			got, err := fn("mongodb-exporter-missing-container-test")

			require.ErrorIs(t, err, errContainerMissing)
			assert.Empty(t, got)
		})
	}
}
