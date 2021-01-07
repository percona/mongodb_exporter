# MongoDB exporter

This is the new MongoDB exporter implementation that handles ALL metrics exposed by MongoDB monitoring commands.
This new implementation loops over all the fields exposed in diagnostic commands and tries to get data from them.

Currently, these metric sources are implemented:
- $collStats
- $indexStats
- getDiagnosticData
- replSetGetStatus
- serverStatus

## Flags
|Flag|Description|Example|
|-----|-----|-----|
|-h, \-\-help|Show context-sensitive help||
|\-\-compatible-mode|Exposes new metrics in the new and old format at the same time||
|\-\-mongodb.collstats-colls|List of comma separated databases.collections to get stats|\-\-mongodb.collstats-colls=testdb.testcol1,testdb.testcol2|
|\-\-mongodb.indexstats-colls|List of comma separated database.collections to get index stats|\-\-mongodb.indexstats-colls=db1.col1,db1.col2|
|\-\-mongodb.dsn|MongoDB connection URI. See https://docs.mongodb.com/manual/reference/connection-string/ for details and available options |\-\-mongodb.dsn=mongodb://user:pass@127.0.0.1:27017/admin?ssl=true|
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
