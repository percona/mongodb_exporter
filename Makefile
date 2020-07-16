.PHONY: all build clean default help init test format check-license
default: help

GO_TEST_PATH ?= ./...
GO_TEST_EXTRA ?=
GO_TEST_COVER_PROFILE ?= cover.out
GO_TEST_CODECOV ?=

TOP_DIR ?= $(shell git rev-parse --show-toplevel)
VERSION ?= $(shell git describe --abbrev=0)
BUILD ?= $(shell date +%FT%T%z)
GOVERSION ?= $(shell go version | cut -d " " -f3)
COMMIT ?= $(shell git rev-parse HEAD)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
GO_BUILD_LDFLAGS = -X main.Version=${VERSION} -X main.Build=${BUILD} -X main.Commit=${COMMIT} -X main.Branch=${BRANCH} -X main.GoVersion=${GOVERSION} -s -w
NAME ?= mnogo_exporter
REPO ?= percona/$(NAME)
GORELEASER_FLAGS ?=
UID ?= $(shell id -u)

export TEST_PSMDB_VERSION?=3.6
export TEST_MONGODB_FLAVOR?=percona/percona-server-mongodb
export TEST_MONGODB_ADMIN_USERNAME?=
export TEST_MONGODB_ADMIN_PASSWORD?=
export TEST_MONGODB_USERNAME?=
export TEST_MONGODB_PASSWORD?=
export TEST_MONGODB_S1_RS?=rs1
export TEST_MONGODB_STANDALONE_PORT?=27017
export TEST_MONGODB_S1_PRIMARY_PORT?=17001
export TEST_MONGODB_S1_SECONDARY1_PORT?=17002
export TEST_MONGODB_S1_SECONDARY2_PORT?=17003
export TEST_MONGODB_S2_RS?=rs2
export TEST_MONGODB_S2_PRIMARY_PORT?=17004
export TEST_MONGODB_S2_SECONDARY1_PORT?=17005
export TEST_MONGODB_S2_SECONDARY2_PORT?=17006
export TEST_MONGODB_CONFIGSVR_RS?=csReplSet
export TEST_MONGODB_CONFIGSVR1_PORT?=17007
export TEST_MONGODB_CONFIGSVR2_PORT?=17008
export TEST_MONGODB_CONFIGSVR3_PORT?=17009
export TEST_MONGODB_MONGOS_PORT?=17000

define TEST_ENV
	TEST_MONGODB_ADMIN_USERNAME=$(TEST_MONGODB_ADMIN_USERNAME) \
	TEST_MONGODB_ADMIN_PASSWORD=$(TEST_MONGODB_ADMIN_PASSWORD) \
	TEST_MONGODB_USERNAME=$(TEST_MONGODB_USERNAME) \
	TEST_MONGODB_PASSWORD=$(TEST_MONGODB_PASSWORD) \
	TEST_MONGODB_S1_RS=$(TEST_MONGODB_S1_RS) \
	TEST_MONGODB_STANDALONE_PORT=$(TEST_MONGODB_STANDALONE_PORT) \
	TEST_MONGODB_S1_PRIMARY_PORT=$(TEST_MONGODB_S1_PRIMARY_PORT) \
	TEST_MONGODB_S1_SECONDARY1_PORT=$(TEST_MONGODB_S1_SECONDARY1_PORT) \
	TEST_MONGODB_S1_SECONDARY2_PORT=$(TEST_MONGODB_S1_SECONDARY2_PORT) \
	TEST_MONGODB_S2_RS=$(TEST_MONGODB_S2_RS) \
	TEST_MONGODB_S2_PRIMARY_PORT=$(TEST_MONGODB_S2_PRIMARY_PORT) \
	TEST_MONGODB_S2_SECONDARY1_PORT=$(TEST_MONGODB_S2_SECONDARY1_PORT) \
	TEST_MONGODB_S2_SECONDARY2_PORT=$(TEST_MONGODB_S2_SECONDARY2_PORT) \
	TEST_MONGODB_CONFIGSVR_RS=$(TEST_MONGODB_CONFIGSVR_RS) \
	TEST_MONGODB_CONFIGSVR1_PORT=$(TEST_MONGODB_CONFIGSVR1_PORT) \
	TEST_MONGODB_CONFIGSVR2_PORT=$(TEST_MONGODB_CONFIGSVR2_PORT) \
	TEST_MONGODB_CONFIGSVR3_PORT=$(TEST_MONGODB_CONFIGSVR3_PORT) \
	TEST_MONGODB_MONGOS_PORT=$(TEST_MONGODB_MONGOS_PORT) \
	TEST_PSMDB_VERSION=$(TEST_PSMDB_VERSION) \
	TEST_MONGODB_FLAVOR=$(TEST_MONGODB_FLAVOR)
endef

env:
	@echo $(TEST_ENV) | tr ' ' '\n' >.env

init:                       ## Install linters.
	go build -modfile=tools/go.mod -o bin/goimports golang.org/x/tools/cmd/goimports
	go build -modfile=tools/go.mod -o bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint
	go build -modfile=tools/go.mod -o bin/reviewdog github.com/reviewdog/reviewdog/cmd/reviewdog

build:                      ## Build the binaries.
	goreleaser --snapshot --skip-publish --rm-dist


FILES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

format:                     ## Format source code.
	go mod tidy
	gofmt -w -s $(FILES)
	bin/goimports -local github.com/Percona-Lab/mnogo_exporter -l -w $(FILES)

check:                      ## Run checks/linters
	bin/golangci-lint run

check-license:              ## Check license in headers.
	@go run .github/check-license.go

help:                       ## Display this help message.
	@echo "Please use \`make <target>\` where <target> is one of:"
	@grep '^[a-zA-Z]' $(MAKEFILE_LIST) | \
	awk -F ':.*?## ' 'NF==2 {printf "  %-26s%s\n", $$1, $$2}'

test: env                   ## Run all tests.
	go test -v -timeout 30s ./...

test-cluster: env           ## Starts MongoDB test cluster.
	docker-compose up -d

test-cluster-clean: env     ## Stops MongoDB test cluster.
	docker-compose down
