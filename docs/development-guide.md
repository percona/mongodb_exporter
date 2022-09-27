# Development Guide

- [Development Guide](#development-guide)
  - [Prerequisite knowledge](#prerequisite-knowledge)
    - [Environment setup](#environment-setup)
    - [Branching workflow](#branching-workflow)
  - [Development](#development)
    - [Using the Makefile](#using-the-makefile)
    - [Kickstart development](#kickstart-development)
    - [Coding guidelines and suggestions](#coding-guidelines-and-suggestions)
  - [Tests](#tests)
    - [Starting the sandbox](#starting-the-sandbox)
    - [Running tests](#running-tests)
  - [Submitting a Pull Request](#submitting-a-pull-request)
    - [Sign the CLA](#sign-the-cla)
    - [Code of Conduct](#code-of-conduct)
    - [PR Checklist](#pr-checklist)
  - [Pull Request is merged](#pull-request-is-merged)

## Prerequisite knowledge

### Environment setup

For developing MongoDB exporter, you need following software installed in your development environment:

- [Golang toolchain](https://go.dev/doc/install) (*v1.17 or higher*)
- `make` utility (*to run Makefile*)
- [Docker](https://docs.docker.com/engine/install/)  (*to run containers*)
- `docker-compose` utlity (*generates local test cluster*)

### Branching workflow

MongoDB exporter uses [semantic versioning](https://semver.org/spec/v2.0.0.html) (**[MAJOR].[MINOR].[PATCH]**):

- MAJOR version when you make incompatible changes
- MINOR version when you add functionality in a backwards compatible manner
- PATCH version when you make backwards compatible bug fixes

All Pull Requests(**PR**) are merged only to `main` branch.

A release is made by forking from `main`. Release branches follow the naming convention of `release-X.Y` (for release version **vX.Y.Z**) . Release branch accepts new commits only to get critical patches or to create a _patch_ release. Release branch is locked after new minor or patch release. The last commit for a new release **vX.Y.Z** gets tagged with `vX.Y.Z` tag in the release branch.

**Please submit your changes only against `main` branch**. If the fix is needed for a patch or minor release, please ask maintainers to cherry pick it into the release branch.

## Development

### Using the Makefile

The root directory has a `Makefile` to help you with development and testing tasks. Use `make` without parameters to get help.

Following options are available:

|Command|Description|
|-----|-----|
|init|Install tools for checks and linting|
|build|Build the exporter binary|
|format|Format source code|
|check|Run checks/linters|
|check-license|Check license in headers|
|help|Display Makefile usage|
|test|Run all tests|
|test-cluster|Start MongoDB test cluster. Use environment variable `TEST_MONGODB_IMAGE` to set flavor and version. Example: `TEST_MONGODB_IMAGE=mongo:4.4 make test-cluster`|
|test-cluster-clean|Stop MongoDB test cluster|

**Please take care to do following**:

- Before using any linting or check utility, invoke `make init` to generate the required utilities.
- Before running any test, invoke `make test-cluster` to start test sandbox

### Kickstart development

Ensure you have Golang toolchain and Docker installed on your system. Once it is done, we need to install tools to format, test and build the exporter. Run following command for it:

```
make init
```
It will install `gci` , `gofumpt` , `golangci-lint` and `reviewdog`. Once we have these tools, we are ready to make changes in the codebase.

### Coding guidelines and suggestions

We use [Effective Go](https://go.dev/doc/effective_go) as our coding standard. Please refer to it in case of any doubts about conventions.

Format the code according to the standards using:

```
make format
```

For any new functionality added to the exporter, please add relevant tests to the test suite.

## Tests

### Starting the sandbox

Docker based containers are created to generate test sandbox. Exporter functionality can be tested in different configurations of MongoDB servers using the sandbox. Makefile in the root directory of the project can be used to create or teardown the sandbox.

Start the sandbox using:

```
make test-cluster
```

Teardown the sandbox using:

```
make test-cluster-clean
```

By default, we create containers using **MongoDB 4.2** base image. You can specify a different version or flavor of MongoDB by specifying a different base image using `TEST_MONGODB_IMAGE` as follows:

```
TEST_MONGODB_IMAGE=mongo:5.0 make test-cluster
```

The sandbox creates following containers:

- 1 standalone MongoDB(`mongod`) server instance (*port exposed locally at **27017***)
- 1 MongoDB router(`mongos`) instance (*port exposed locally at **17000***)
- First replica-set of MongoDB with 3 servers and an arbiter instances (*port exposed locally at **17001**, **17002**, **17003** for servers and **17011** for arbiter*)
- Second replica-set of MongoDB with 3 servers and an arbiter instances (*port exposed locally at **17004**, **17005**, **17006** for servers and **17012** for arbiter*)
- 1 replica-sets of configuration server instance (*port exposed locally at **17007**, **17008**, **17009***)

The containers created in sandbox can be used to test following configurations for MongoDB:

- Standalone server
- Replica-set of servers
- Sharded cluster of servers

All instances are created without user and password. MongoDB shell (`mongosh` or legacy `mongo`) can be used locally to connect to MongoDB instance running inside the containers.

For example, connect to MongoDB router(`mongos`) using:

```
mongosh mongodb://127.0.0.1:17000
```

### Running tests

All tests in exporter codebase are integration tests, intended to run against test sandbox. Hence, **ensure test sandbox is running before invoking test suite**. To run the test suite, invoke following command:

```
make test
```

## Submitting a Pull Request

Before submitting contributions via a PR, you should first complete the following prerequisites.

### Sign the CLA

Before you can contribute, please sign our [Contributor License Agreement](https://cla-assistant.percona.com/percona/mongodb_exporter) (CLA). You can do this using your GitHub account and one click.

### Code of Conduct

Please make sure to read and agree to our [Code of Conduct](https://github.com/percona/community/blob/main/content/contribute/coc.md).

### PR Checklist

Before submitting a PR, please use following checklist to avoid common issues:

- [ ] [Format code](#coding-guidelines-and-suggestions) as per our coding standards
- [ ] Add new tests (if applicable)
- [ ] [Run test suite](#running-tests)
- [ ] Capture screenshots of new behavior (if applicable)

Once you have used above checklist, go ahead and file a PR against `main` branch of the project.

## Pull Request is merged

Once your pull request is merged, you are an official Percona Community Contributor.

Welcome to the community! Thanks again for your contribution.