# Percona MongoDB Exporter

[![Release](https://github-release-version.herokuapp.com/github/percona/mongodb_exporter/release.svg?style=flat)](https://github.com/percona/mongodb_exporter/releases/latest)
[![Build Status](https://travis-ci.org/percona/mongodb_exporter.svg?branch=master)](https://travis-ci.org/percona/mongodb_exporter)
[![Go Report Card](https://goreportcard.com/badge/github.com/percona/mongodb_exporter)](https://goreportcard.com/report/github.com/percona/mongodb_exporter)
[![CLA assistant](https://cla-assistant.io/readme/badge/percona/mongodb_exporter)](https://cla-assistant.io/percona/mongodb_exporter)

Based on [MongoDB exporter](https://github.com/dcu/mongodb_exporter) by David Cuadrado (@dcu), but forked for full sharded support and structure changes.

## Features

- MongoDB Server Status metrics (*cursors, operations, indexes, storage, etc*)
- MongoDB Replica Set metrics (*members, ping, replication lag, etc*)
- MongoDB Replication Oplog metrics (*size, length in time, etc*)
- MongoDB Sharding metrics (*shards, chunks, db/collections, balancer operations*)
- MongoDB RocksDB storage-engine metrics (*levels, compactions, cache usage, i/o rates, etc*)
- MongoDB WiredTiger storage-engine metrics (*cache, blockmanger, tickets, etc*)
- MongoDB Top Metrics per collection (writeLock, readLock, query, etc*)


## Building and running

### Building

```bash
make
```


### Running

To define your own MongoDB URL, use environment variable `MONGODB_URL`. If set this variable takes precedence over `-mongodb.uri` flag.

To enable HTTP basic authentication, set environment variable `HTTP_AUTH` to user:password pair. Alternatively, you can
use YAML file with `server_user` and `server_password` fields.

```bash
export MONGODB_URL='mongodb://localhost:27017'
export HTTP_AUTH='user:password'
./mongodb_exporter <flags>
```

### Flags

See the help page with `-h`.

If you use [MongoDB Authorization](https://docs.mongodb.org/manual/core/authorization/), you must:

1. Create a user with '*clusterMonitor*' role and '*read*' on the '*local*' database, like the following (*replace username/password!*):

```js
db.getSiblingDB("admin").createUser({
    user: "mongodb_exporter",
    pwd: "s3cr3tpassw0rd",
    roles: [
        { role: "clusterMonitor", db: "admin" },
        { role: "read", db: "local" }
    ]
})
```

2. Set environment variable `MONGODB_URL` before starting the exporter:

```bash
export MONGODB_URL=mongodb://mongodb_exporter:s3cr3tpassw0rd@localhost:27017
```

If you use [x.509 Certificates to Authenticate Clients](https://docs.mongodb.com/manual/tutorial/configure-x509-client-authentication/), pass in username and `authMechanism` via [connection options](https://docs.mongodb.com/manual/reference/connection-string/#connections-connection-options) to the MongoDB uri. Eg:

```
mongodb://CN=myName,OU=myOrgUnit,O=myOrg,L=myLocality,ST=myState,C=myCountry@localhost:27017/?authMechanism=MONGODB-X509
```

## Note about how this works

Point the process to any mongo port and it will detect if it is a mongos, replicaset member, or stand alone mongod and return the appropriate metrics for that type of node. This was done to prevent the need to an exporter per type of process.

## Roadmap

- Document more configurations options here
- Stabilize RocksDB and WiredTiger support
- Move MongoDB user/password/authdb to a file (for security)
- Write more go tests


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
