module github.com/gaantunes/mongodb_exporter/v1

go 1.15

// Update percona-toolkit with `go get -v github.com/percona/percona-toolkit@3.0; go mod tidy` (without `-u`)
// until we have everything we need in a tagged release.

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/alecthomas/kong v0.2.17
	github.com/percona/exporter_shared v0.7.3
	github.com/percona/mongodb_exporter v0.20.6
	github.com/percona/percona-toolkit v0.0.0-20210503082911-610ab8c66a81
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	go.mongodb.org/mongo-driver v1.5.3
)
