# Percona Mongodb Exporter

Based on MongoDB exporter for prometheus.io, written in go (https://github.com/dcu/mongodb_exporter), but forked for full sharded support and structure changes.

### Experimental

The exporter is in beta/experimental state and field names are **very likely to change** and features may change or get removed!

### Features

- MongoDB Server Status metrics (*cursors, operations, indexes, storage, etc*)
- MongoDB Replica Set metrics (*members, ping, replication lag, etc*)
- MongoDB Replication Oplog metrics (*size, length in time, etc*)
- MongoDB Sharding metrics (*shards, chunks, db/collections, balancer operations*)
- MongoDB RocksDB storage-engine metrics (*levels, compactions, cache usage, i/o rates, etc*)
- MongoDB WiredTiger storage-engine metrics (*cache, blockmanger, tickets, etc*)

### Building

    go get -d github.com/percona/mongodb_exporter
    cd $GOPATH/src/github.com/percona/mongodb_exporter
    make

### Usage

The exporter can be started by running the '*mongodb_exporter*' binary that is created in the build step. The exporter will try to connect to '*mongodb://localhost:27017*' (no auth) as default if no options are supplied.

It is recommended to define the following options:

- **-web.listen-address** - The listen address of the exporter (*default: ":9104"*)
- **-log_dir** - The directory to write the log file (*default: /tmp*)

To define your own MongoDB URL, use environment variable `MONGODB_URL`. If set this variable takes precedence over **-mongodb.uri** flag.
For example: `export MONGODB_URL=mongodb://localhost:27017`

To enable HTTP basic authentication, set environment variable `HTTP_AUTH` to user:password pair.
For example: `export HTTP_AUTH="user:password"`

*For more options see the help page with '-h' or '--help'*

If you use [MongoDB Authorization](https://docs.mongodb.org/manual/core/authorization/), you must:

1. Create a user with '*clusterMonitor*' role and '*read*' on the '*local*' database, like the following (*replace username/password!*):

```
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

```
export MONGODB_URL=mongodb://mongodb_exporter:s3cr3tpassw0rd@localhost:27017
```

### Note about how this works
Point the process to any mongo port and it will detect if it is a mongos, replicaset member, or stand alone mongod and return the appropriate metrics for that type of node. This was done to preent the need to an exporter per type of process.

### Roadmap

- Document more configurations options here
- Stabilize RocksDB and WiredTiger support (*currently beta/experimental*)
- Move MongoDB user/password/authdb to a file (for security)
- Write more go tests
- Version scheme

### Contact

- David Murphy - [Twitter](https://twitter.com/dmurphy_data) / [Github](https://github.com/dbmurphy) / [Email](mailto:david.murphy@percona.com)
- Tim Vaillancourt - [Github](https://github.com/timvaillancourt) / [Email](mailto:tim.vaillancourt@percona.com)
- Percona - [Twitter](https://twitter.com/Percona) / [Contact Page](https://www.percona.com/about-percona/contact)
