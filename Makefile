VERSION=$(shell cat VERSION)
GIT_COMMIT=$(shell git rev-parse HEAD)
GO_BUILD_LDFLAGS="-X main.version=$(VERSION) -X main.versionGitCommit=$(GIT_COMMIT)"

all: build

build: mongodb_exporter

vendor: glide.*
	go get github.com/Masterminds/glide
	glide install

mongodb_exporter: vendor *.go collector/*.go collector/*/*.go shared/*.go VERSION
	go build -ldflags $(GO_BUILD_LDFLAGS) -o mongodb_exporter mongodb_exporter.go

clean:
	rm -rf mongodb_exporter vendor 2>/dev/null || true
