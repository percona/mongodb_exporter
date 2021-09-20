module github.com/percona/mongodb_exporter

go 1.15

// Update percona-toolkit with `go get -v github.com/percona/percona-toolkit@3.0; go mod tidy` (without `-u`)
// until we have everything we need in a tagged release.

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/alecthomas/kong v0.2.17
	github.com/percona/exporter_shared v0.7.3
	github.com/percona/percona-toolkit v0.0.0-20210803120725-d14d18a1bfb6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	github.com/shirou/gopsutil v3.21.8+incompatible // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	go.mongodb.org/mongo-driver v1.5.3
)
