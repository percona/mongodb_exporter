module github.com/percona/mongodb_exporter

go 1.14

// Update percona-toolkit with `go get -v github.com/percona/percona-toolkit@3.0; go mod tidy` (without `-u`)
// until we have everything we need in a tagged release.

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/alecthomas/kong v0.2.11
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/percona/exporter_shared v0.7.3-0.20200915121748-415d485ed1a8
	github.com/percona/percona-toolkit v0.0.0-20200908164809-0aac7b4cfc30
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.13.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	go.mongodb.org/mongo-driver v1.4.1
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)
