# MongoDB exporter
[![Release](https://img.shields.io/github/release/percona/mongodb_exporter.svg?style=flat)](https://github.com/percona/mongodb_exporter/releases/latest)
[![Build Status](https://github.com/percona/mongodb_exporter/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/percona/mongodb_exporter/actions/workflows/go.yml?query=branch%3Amain)
[![codecov.io Code Coverage](https://img.shields.io/codecov/c/github/percona/mongodb_exporter.svg?maxAge=2592000)](https://codecov.io/github/percona/mongodb_exporter?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/percona/mongodb_exporter)](https://goreportcard.com/report/github.com/percona/mongodb_exporter)
[![CLA assistant](https://cla-assistant.percona.com/readme/badge/percona/mongodb_exporter)](https://cla-assistant.percona.com/percona/mongodb_exporter)

This is the new MongoDB exporter implementation that handles ALL metrics exposed by MongoDB monitoring commands.
This new implementation loops over all the fields exposed in diagnostic commands and tries to get data from them.

Currently, these metric sources are implemented:
- $collStats
- $indexStats
- getDiagnosticData
- replSetGetStatus
- serverStatus

| Old Percona MongoDB exporter                                                  |
|:------------------------------------------------------------------------------|
| old 0.1x.y version (ex `master` branch) is moved to the `release_0.1x` branch |


## Flags
|Flag|Description|Example|
|-----|-----|-----|
|-h, \-\-help|Show context-sensitive help||
|\-\-compatible-mode|Exposes new metrics in the new and old format at the same time||
|\-\-mongodb.collstats-colls|List of comma separated databases.collections to get stats|\-\-mongodb.collstats-colls=testdb.testcol1,testdb.testcol2|
|\-\-mongodb.indexstats-colls|List of comma separated database.collections to get index stats|\-\-mongodb.indexstats-colls=db1.col1,db1.col2|
|\-\-mongodb.dsn|MongoDB connection URI|\-\-mongodb.dsn=mongodb://user:pass@127.0.0.1:27017/admin?ssl=true|
|\-\-expose-path|Metrics expose path|\-\-expose-path=/metrics_new|
|\-\-expose-port|HTTP expose server port|\-\-expose-port=9216|
|-D, --debug|Enable debug mode||
|--version|Show version and exit|

 ### Build the exporter
The build process uses the dockerized version of goreleaser so you don't need to install Go.
Just run `make build` and the new binaries will be generated under the build directory.
```
├── build
│ ├── config.yaml
│ ├── mongodb_exporter_7c73946_checksums.txt
│ ├── mongodb_exporter-7c73946.darwin-amd64.tar.gz
│ ├── mongodb_exporter-7c73946.linux-amd64.tar.gz
│ ├── mongodb_exporter_darwin_amd64
│ │ └── mongodb_exporter <--- MacOS binary
│ └── mongodb_exporter_linux_amd64
│ └── mongodb_exporter <--- Linux binary
```

### Build the exporter
The build process uses the dockerized version of goreleaser so you don't need to install Go.
Just run `make build` and the new binaries will be generated under the build directory.
```
├── build
│ ├── config.yaml
│ ├── mongodb_exporter_7c73946_checksums.txt
│ ├── mongodb_exporter-7c73946.darwin-amd64.tar.gz
│ ├── mongodb_exporter-7c73946.linux-amd64.tar.gz
│ ├── mongodb_exporter_darwin_amd64
│ │ └── mongodb_exporter <--- MacOS binary
│ └── mongodb_exporter_linux_amd64
│ └── mongodb_exporter <--- Linux binary
```

### Running the exporter
If you built the exporter using the method mentioned in the previous section, the generated binaries are in `mongodb_exporter_linux_amd64/mongodb_exporter` or `mongodb_exporter_darwin_amd64/mongodb_exporter`

#### Example
```
mongodb_exporter_linux_amd64/mongodb_exporter --mongodbdsn=mongodb://127.0.0.1:17001
```
#### Enabling collstats metrics gathering
`--mongodb.collstats-colls` receives a list of databases and collections to monitor using collstats.
Usage example: `--mongodb.collstats-colls=database1.collection1,database2.collection2`
```
mongodb_exporter_linux_amd64/mongodb_exporter --mongodbdsn=mongodb://127.0.0.1:17001 --mongodb.collstats-colls=db1.c1,db2.c2
```
#### Enabling compatibility mode.
When compatibility mode is enabled by the `--compatible-mode`, the exporter will expose all new metrics with the new naming and labeling schema and at the same time will expose metrics in the version 1 compatible way.
For example, if compatibility mode is enabled, the metric `mongodb_ss_wt_log_log_bytes_written` (new format)
```
# HELP mongodb_ss_wt_log_log_bytes_written serverStatus.wiredTiger.log.
# TYPE mongodb_ss_wt_log_log_bytes_written untyped
mongodb_ss_wt_log_log_bytes_written 2.6208e+06
```
will be also exposed as `mongodb_mongod_wiredtiger_log_bytes_total`  with the `unwritten` label.
```
HELP mongodb_mongod_wiredtiger_log_bytes_total mongodb_mongod_wiredtiger_log_bytes_total
# TYPE mongodb_mongod_wiredtiger_log_bytes_total untyped
mongodb_mongod_wiredtiger_log_bytes_total{type="unwritten"} 2.6208e+06
```



## Submitting Bug Reports

If you find a bug in Percona MongoDB Exporter or one of the related projects, you should submit a report to that project's [JIRA](https://jira.percona.com) issue tracker.

Your first step should be [to search](https://jira.percona.com/issues/?jql=project=PMM%20AND%20component=MongoDB_Exporter) the existing set of open tickets for a similar report. If you find that someone else has already reported your problem, then you can upvote that report to increase its visibility.

If there is no existing report, submit a report following these steps:

1. [Sign in to Percona JIRA.](https://jira.percona.com/login.jsp) You will need to create an account if you do not have one.
2. [Go to the Create Issue screen and select the relevant project.](https://jira.percona.com/secure/CreateIssueDetails!init.jspa?pid=11600&issuetype=1&priority=3&components=11603)
3. Fill in the fields of Summary, Description, Steps To Reproduce, and Affects Version to the best you can. If the bug corresponds to a crash, attach the stack trace from the logs.

An excellent resource is [Elika Etemad's article on filing good bug reports.](http://fantasai.inkedblade.net/style/talks/filing-good-bugs/).

As a general rule of thumb, please try to create bug reports that are:

- *Reproducible.* Include steps to reproduce the problem.
- *Specific.* Include as much detail as possible: which version, what environment, etc.
- *Unique.* Do not duplicate existing tickets.
- *Scoped to a Single Bug.* One bug per report.
