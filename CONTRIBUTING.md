# Welcome to MongoDB exporter!

We're glad that you would like to become a Percona community member and participate in keeping open source open.
This is the new MongoDB exporter implementation that handles ALL metrics exposed by MongoDB monitoring commands.
This implementation loops over all the fields exposed in diagnostic commands and tries to get data from them.

## Prerequisites

Before submitting code or documentation contributions, you should first complete the following prerequisites.

### 1. Sign the CLA

Before you can contribute, we kindly ask you to sign our [Contributor License Agreement](https://cla-assistant.percona.com/percona/mongodb_exporter) (CLA). You can do this using your GitHub account and one click.

### 2. Code of Conduct

Please make sure to read and agree to our [Code of Conduct](https://github.com/percona/community/blob/main/content/contribute/coc.md).

## Submitting a Bug

If you find a bug in Percona MongoDB Exporter and it is not related to PMM, please open issue in [GitHub new issue](https://github.com/percona/mongodb_exporter/issues/new/choose), if you use PMM, please submit a report to that project's [JIRA](https://jira.percona.com/projects/PMM/issues) issue tracker.

Your first step should be to search [GH issues](https://github.com/percona/mongodb_exporter/issues) and [JIRA PMM issues](https://jira.percona.com/issues/?jql=project=PMM%20AND%20component=MongoDB_Exporter) for the existing set of open tickets for a similar report. If you find that someone else has already reported your problem, then you can upvote that report to increase its visibility.

### Submitting PMM Bug

If there is no existing PMM report for the issue that relates to PMM, submit a report following these steps:

1. [Sign in to Percona JIRA.](https://jira.percona.com/login.jsp) You will need to create an account if you do not have one.
2. [Go to the Create Issue screen and select the relevant project.](https://jira.percona.com/secure/CreateIssueDetails!init.jspa?pid=11600&issuetype=1&priority=3&components=11603)
3. Fill in the fields of Summary, Description, Steps To Reproduce, and Affects Version to the best you can. If the bug corresponds to a crash, attach the stack trace from the logs.

An excellent resource is [Elika Etemad's article on filing good bug reports.](https://fantasai.inkedblade.net/style/talks/filing-good-bugs/).

As a general rule of thumb, please try to create bug reports that are:

- _Reproducible._ Include steps to reproduce the problem.
- _Specific._ Include as much detail as possible: which version, what environment, etc.
- _Unique._ Do not duplicate existing tickets.
- _Scoped to a Single Bug._ One bug per report.

## Branching workflow

MongoDB exporter uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html) `[major].[minor].[patch]`:

- major version when you make incompatible changes
- minor version when you add functionality in a backwards compatible manner
- patch version when you make backwards compatible bug fixes

`main` is a main branch where all the changes are merged **first**.

`release-x.y` are the release branches and are forked from `main` to prepare and test the release. Release branch stays open to get critical patches and to release another _patch_ release. Release branch closes (no merges happens) just after new minor release is created.

Each release is tagged with `vx.y.z` tag.

Please submit your changes against `main` branch, if the fix is needed for the patch or minor release - please ask maintainers to cherry pick it into the release branch.

## Setup your local development environment

### Using the Makefile

In the main directory there is a `Makefile` to help you with development and testing tasks.
Use `make` without parameters to get help.
These are these available options:
|Command|Description|
|-----|-----|
|init|Install linters|
|build|Build the binaries|
|format|Format source code|
|check|Run checks/linters|
|check-license|Check license in headers. |
|help|Display this help message. |
|test|Run all tests (need to start the sandbox first)|
|test-cluster|Starts MongoDB test cluster. Use `env var TEST_MONGODB_IMAGE` to set flavor and version. Example:|
| |`TEST_MONGODB_IMAGE=mongo:3.6 make test-cluster`|
|test-cluster-clean|Stops MongoDB test cluster|

### Initializing the development environment

First you need to have `Go` and `Docker` installed on your system and then, in order to install tools to format, test and build the exporter, you need to run this command:

```
make init
```

It will install `goimports`, `goreleaser`, `golangci-lint` and `reviewdog`.

## Tests

### Starting the sandbox

The testing sandbox starts `n` MongoDB instances as follows:

- 3 Instances for shard 1 at ports 17001, 17002, 17003
- 3 instances for shard 2 at ports 17004, 17005, 17006
- 3 config servers at ports 17007, 17008, 17009
- 1 mongos server at port 17000
- 1 stand alone instance at port 27017

All instances are currently running without user and password so for example, to connect to the **mongos** you can just use:

```
mongo mongodb://127.0.0.1:17001/admin
```

The sandbox can be started using the provided Makefile using: `make test-cluster` and it can be stopped using `make test-cluster-clean`.

### Running tests

To run the unit tests, just run `make test`.

## Submitting a Pull Request

### Formatting code

Before submitting code, please run `make format` to format the code according to the standards.
Submit your code against the `main` branch.

### Code Reviews

After submitting your PR please add `pmm-review-exporters` team as a reviewer - that would auto assign reviewers to review your PR.

## After your Pull Request is merged

Once your pull request is merged, you are an official Percona Community Contributor. Welcome to the community!

We're looking forward to your contributions and hope to hear from you soon on our [Forums](https://forums.percona.com).
