default: help

GO_TEST_PATH?=./...
GO_TEST_EXTRA?=
GO_TEST_COVER_PROFILE?=cover.out
GO_TEST_CODECOV?=

TOP_DIR=$(shell git rev-parse --show-toplevel)
VERSION ?=$(shell git describe --abbrev=0)
BUILD ?=$(shell date +%FT%T%z)
GOVERSION ?=$(shell go version | cut -d " " -f3)
COMMIT ?=$(shell git rev-parse HEAD)
BRANCH ?=$(shell git rev-parse --abbrev-ref HEAD)

GO_BUILD_LDFLAGS=-X main.Version=${VERSION} -X main.Build=${BUILD} -X main.Commit=${COMMIT} -X main.Branch=${BRANCH} -X main.GoVersion=${GOVERSION} -s -w

NAME?=mnogo_exporter
REPO?=percona/$(NAME)
GORELEASER_FLAGS?=
UID?=$(shell id -u)

export TEST_PSMDB_VERSION?=3.6
export TEST_MONGODB_FLAVOR?=percona/percona-server-mongodb
export TEST_MONGODB_ADMIN_USERNAME?=admin
export TEST_MONGODB_ADMIN_PASSWORD?=admin123456
export TEST_MONGODB_USERNAME?=test
export TEST_MONGODB_PASSWORD?=123456
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

FILES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

init:						## Install linters
	curl https://raw.githubusercontent.com/reviewdog/reviewdog/master/install.sh| sh -s
	curl https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s latest
	go get golang.org/x/tools/cmd/goimports

build:						## Build the binaries
	goreleaser --snapshot --skip-publish --rm-dist

format:						## Format source code.
	go get golang.org/x/tools/cmd/goimports 
	gofmt -w -s $(FILES)
	goimports -local github.com/Percona-Lab/mnogo_exporter -l -w $(FILES)

help:						## Display this help message.
	@echo "Please use \`make <target>\` where <target> is one of:"
	@grep '^[a-zA-Z]' $(MAKEFILE_LIST) | \
	awk -F ':.*?## ' 'NF==2 {printf "  %-26s%s\n", $$1, $$2}'

test: env					## Run all tests
	go test -timeout 30s ./...

certs:					   	## Generate SSL certificates for the MongoDB sandbox
	docker/test/gen-certs/gen-certs.sh

test-cluster: env certs	   	## Starts MongoDB test cluster 
	TEST_PSMDB_VERSION=$(TEST_PSMDB_VERSION) \
	docker-compose up \
	--detach \
	--force-recreate \
	--always-recreate-deps \
	--renew-anon-volumes \
	init
	docker/test/init-cluster-wait.sh

test-cluster-clean: env		## Stops MongoDB test cluster
	docker-compose down -v
