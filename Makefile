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
BIN_NAME	:= mongodb_exporter

PREFIX              ?= $(shell pwd)
BIN_DIR             ?= $(PREFIX)/dist
DOCKER_IMAGE_NAME   ?= mongodb-exporter
DOCKER_IMAGE_TAG    ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

# Race detector is only supported on amd64.
RACE := $(shell test $$(go env GOARCH) != "amd64" || (echo "-race"))

export TRAVIS_APP_HOST ?= $(shell hostname)
export TRAVIS_BRANCH   ?= $(shell git rev-parse --abbrev-ref HEAD)
export TRAVIS_TAG 	   ?= $(shell git describe --tags --abbrev=0)
export GO_PACKAGE 	   := github.com/percona/mongodb_exporter
export APP_VERSION	   := $(shell echo $(TRAVIS_TAG) | sed -e 's/v//g')
export APP_REVISION    := $(shell git show --format='%H' HEAD -q)
export BUILD_TIME	   := $(shell date '+%Y%m%d-%H:%M:%S')

all: clean format test build

style:
	@echo ">> checking code style"
	@! gofmt -s -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

test:
	@echo ">> running tests"
	gocoverutil -coverprofile=coverage.txt test -short -v $(RACE) $(pkgs)

testall:
	@echo ">> running all tests"
	gocoverutil -coverprofile=coverage.txt test -v $(RACE) $(pkgs)

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

build: init
	@echo ">> building binaries"
	CGO_ENABLED=0 @$(GO) build -v -a \
		-tags 'netgo' \
		-ldflags '\
		-X $(GO_PACKAGE)/vendor/github.com/prometheus/common/version.Version=$(APP_VERSION) \
    	-X $(GO_PACKAGE)/vendor/github.com/prometheus/common/version.Revision=$(GIT_REVISION) \
    	-X $(GO_PACKAGE)/vendor/github.com/prometheus/common/version.Branch=$(TRAVIS_BRANCH) \
    	-X $(GO_PACKAGE)/vendor/github.com/prometheus/common/version.BuildUser=$(USER)@$(TRAVIS_APP_HOST) \
    	-X $(GO_PACKAGE)/vendor/github.com/prometheus/common/version.BuildDate=$(BUILD_TIME) \
		'\
	 	-o $(BIN_DIR)/$(BIN_NAME) .

snapshot: init
	@echo ">> building snapshot"
	goreleaser --snapshot --skip-sign --skip-validate --skip-publish --rm-dist

release: vendor
	@echo ">> building release"
	goreleaser release --rm-dist

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

$(GOPATH)/bin/dep:
	curl -s https://raw.githubusercontent.com/golang/dep/v0.5.0/install.sh | sh

$(GOPATH)/bin/gocoverutil:
	$(GO) get -u github.com/AlekSi/gocoverutil

$(GOPATH)/bin/goreleaser:
	curl -sL https://git.io/goreleaser | sh -s -- --version && cp $(TMPDIR)/goreleaser $(GOPATH)/bin/goreleaser

init: $(GOPATH)/bin/dep $(GOPATH)/bin/gocoverutil $(GOPATH)/bin/goreleaser

# Ensure that vendor/ is in sync with code and Gopkg.*
check-vendor-synced: init
	rm -fr vendor/
	dep ensure -v
	git diff --exit-code

clean:
	@echo ">> removing build artifacts"
	@rm -f $(PREFIX)/coverage.txt
	@rm -Rf $(PREFIX)/dist


.PHONY: init all style format build test vet release docker clean check-vendor-synced
