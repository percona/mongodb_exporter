# Copyright 2015 The Prometheus Authors
# Copyright 2017 Percona LLC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO          := go
GOPATH      := $(shell $(GO) env GOPATH)
pkgs		= ./...

PREFIX              ?= $(shell pwd)
BIN_DIR             ?= $(PREFIX)/bin
DOCKER_IMAGE_NAME   ?= mongodb-exporter
DOCKER_IMAGE_TAG    ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

# Race detector is only supported on amd64.
RACE := $(shell test $$(go env GOARCH) != "amd64" || (echo "-race"))

export BIN_NAME        := mongodb_exporter
export TRAVIS_APP_HOST ?= $(shell hostname)
export TRAVIS_BRANCH   ?= $(shell git describe --all --contains --dirty HEAD)
export TRAVIS_TAG      ?= $(shell git describe --tags --abbrev=0)
export GO_PACKAGE      := github.com/percona/mongodb_exporter
export APP_VERSION     := $(shell echo $(TRAVIS_TAG) | sed -e 's/v//g')
export APP_REVISION    := $(shell git rev-parse HEAD)
export BUILD_TIME      := $(shell date '+%Y%m%d-%H:%M:%S')

# We sets default pmm version to empty as we want to build community release by default
export PMM_RELEASE_VERSION    ?=
export PMM_RELEASE_TIMESTAMP  = $(shell date '+%s')
export PMM_RELEASE_FULLCOMMIT = $(APP_REVISION)
export PMM_RELEASE_BRANCH     = $(TRAVIS_BRANCH)

MONGODB_IMAGE?=percona/percona-server-mongodb:3.6
TEST_MONGODB_ADMIN_USERNAME?=admin
TEST_MONGODB_ADMIN_PASSWORD?=admin123456
TEST_MONGODB_USERNAME?=test
TEST_MONGODB_PASSWORD?=123456

TEST_MONGODB_CONFIGSVR_RS?=csReplSet
TEST_MONGODB_S1_RS?=rs1
TEST_MONGODB_S2_RS?=rs2
TEST_MONGODB_S3_RS?=rs3

define TEST_ENV
	GOCACHE=$(GOCACHE) \
	TEST_MONGODB_ADMIN_USERNAME=$(TEST_MONGODB_ADMIN_USERNAME) \
	TEST_MONGODB_ADMIN_PASSWORD=$(TEST_MONGODB_ADMIN_PASSWORD) \
	TEST_MONGODB_USERNAME=$(TEST_MONGODB_USERNAME) \
	TEST_MONGODB_PASSWORD=$(TEST_MONGODB_PASSWORD) \
	TEST_MONGODB_S1_RS=$(TEST_MONGODB_S1_RS) \
	TEST_MONGODB_S2_RS=$(TEST_MONGODB_S2_RS) \
	TEST_MONGODB_S3_RS=$(TEST_MONGODB_S3_RS) \
	TEST_MONGODB_CONFIGSVR_RS=$(TEST_MONGODB_CONFIGSVR_RS) \
	MONGODB_IMAGE=$(MONGODB_IMAGE)
endef

all: init format style build test-cluster-up test-all docker-image snapshot

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

style:
	@echo ">> checking code style"
	@! gofmt -s -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

# Ensure that vendor/ is in sync with code and Gopkg.*
check-vendor-synced: init
	rm -fr vendor/
	dep ensure -v
	git diff --exit-code

test:
	@echo ">> running tests"
	go test -coverprofile=coverage.txt -short -v $(RACE) $(pkgs)

test-all:
	@echo ">> running all tests"
	go test -coverprofile=coverage.txt -v $(RACE) $(pkgs)

# We use this target name to build binary across all PMM components
# Note: We duplicate "-X" linker parameters for backward compatibility with go1.12
build:
	@echo ">> building binary"
	CGO_ENABLED=0 $(GO) build -v \
		-ldflags "\
		-X $(GO_PACKAGE)/vendor/github.com/percona/pmm/version.ProjectName=$(BIN_NAME) \
		-X $(GO_PACKAGE)/vendor/github.com/percona/pmm/version.Version=$(APP_VERSION) \
		-X $(GO_PACKAGE)/vendor/github.com/percona/pmm/version.PMMVersion=$(PMM_RELEASE_VERSION) \
		-X $(GO_PACKAGE)/vendor/github.com/percona/pmm/version.Timestamp=$(PMM_RELEASE_TIMESTAMP) \
		-X $(GO_PACKAGE)/vendor/github.com/percona/pmm/version.FullCommit=$(PMM_RELEASE_FULLCOMMIT) \
		-X $(GO_PACKAGE)/vendor/github.com/percona/pmm/version.Branch=$(PMM_RELEASE_BRANCH) \
		-X $(GO_PACKAGE)/vendor/github.com/prometheus/common/version.BuildUser=$(USER)@$(TRAVIS_APP_HOST) \
		-X github.com/percona/pmm/version.ProjectName=$(BIN_NAME) \
		-X github.com/percona/pmm/version.Version=$(APP_VERSION) \
		-X github.com/percona/pmm/version.PMMVersion=$(PMM_RELEASE_VERSION) \
		-X github.com/percona/pmm/version.Timestamp=$(PMM_RELEASE_TIMESTAMP) \
		-X github.com/percona/pmm/version.FullCommit=$(PMM_RELEASE_FULLCOMMIT) \
		-X github.com/percona/pmm/version.Branch=$(PMM_RELEASE_BRANCH) \
		-X github.com/prometheus/common/version.BuildUser=$(USER)@$(TRAVIS_APP_HOST) \
		"\
		-o $(BIN_DIR)/$(BIN_NAME) .

docker-image:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

# It's just alias to build binary across all PMM components
release: build

community-release: $(GOPATH)/bin/goreleaser
	@echo ">> building release"
	goreleaser release --rm-dist --skip-validate

snapshot: $(GOPATH)/bin/goreleaser
	@echo ">> building snapshot"
	goreleaser --snapshot --skip-sign --skip-validate --skip-publish --rm-dist

clean:
	@echo ">> removing build artifacts"
	$(MAKE) test-cluster-down
	@rm -f $(PREFIX)/.env
	@rm -f $(PREFIX)/coverage.txt
	@rm -Rf $(PREFIX)/bin
	@rm -Rf $(PREFIX)/dist

test-cluster-up: env
	MONGODB_IMAGE=$(MONGODB_IMAGE) \
	docker-compose up \
	--detach \
	--force-recreate \
	--always-recreate-deps \
	--renew-anon-volumes \
	init
	docker/test/init-cluster-wait.sh

test-cluster-down: env
	docker-compose down -v

env:
	@echo $(TEST_ENV) | tr ' ' '\n' >.env

init: $(GOPATH)/bin/dep $(GOPATH)/bin/goreleaser

$(GOPATH)/bin/dep:
	curl -s https://raw.githubusercontent.com/golang/dep/v0.5.0/install.sh | sh

$(GOPATH)/bin/goreleaser:
	curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | BINDIR=$(GOPATH)/bin sh

.PHONY: init env test-cluster-down test-cluster-up clean check-vendor-synced docker-image snapshot community-release release build vet format test-all test style all
