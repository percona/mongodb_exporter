module github.com/percona/mongodb_exporter

go 1.15

// Update percona-toolkit with `go get -v github.com/percona/percona-toolkit@3.0; go mod tidy` (without `-u`)
// until we have everything we need in a tagged release.

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/alecthomas/kong v0.2.17
	github.com/kr/pretty v0.3.0 // indirect
	github.com/percona/exporter_shared v0.7.3
	github.com/percona/percona-toolkit v0.0.0-20210806171304-d5a509fa7d15
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.1-0.20180714160509-73f8eece6fdc // indirect
	go.mongodb.org/mongo-driver v1.5.3
	golang.org/x/tools v0.0.0-20201023174141-c8cfbd0f21e6 // indirect
	mvdan.cc/gofumpt v0.0.0-20200927160801-5bfeb2e70dd6 // indirect
)
