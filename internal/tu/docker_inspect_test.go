package tu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInspectContainer(t *testing.T) {
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
		assert.NoError(t, err)

		ns := di[0].NetworkSettings.Ports["27017/tcp"][0].HostPort
		assert.Equal(t, ns, tc.wantPort)
	}
}
