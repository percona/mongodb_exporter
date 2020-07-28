# mnogo exporter

This is the new MongoDB exporter implementation that handles ALL metrics exposed by MongoDB monitoring commands.
Currently, these metric sources are implemented:
- $collStats
- getDiagnosticData
- replSetGetStatus
- serverStatus
## Flags
|Flag|Description|Example|
|-----|-----|-----|
|-h, \-\-help|Show context-sensitive help||
|\-\-mongodb.collstats-colls|List of comma separated databases.collections to get stats|\-\-mongodb.collstats-colls=testdb.testcol1,testdb.testcol2|
|\-\-mongodb.dsn|MongoDB connection URI|\-\-mongodb.dsn=mongodb://user:pass@127.0.0.1:27017/admin?ssl=true|
|\-\-expose-path|Metrics expose path|\-\-expose-path=/metrics_new|
|\-\-expose-port|HTTP expose server port|\-\-expose-port=9216|
|-D, --debug|Enable debug mode||
|--version|Show version and exit|

## Initializing the development environment
First you need to have Go installed on your system and then, in order to install tools to format, test and build the exporter, you need to run this command:
```
make init
```
It will install gomports, goreleaser, golangci-lint and reviewdog.

## Testing 
### Initialize tools and dependencies


### Starting the sandbox
The testing sandbox starts n MongoDB instances as follow:
- 3 Instances for shard 1 at ports 17001, 17002, 17003
- 3 instances for shard 2 at ports 17004, 17005, 17006
- 3 config servers at ports 17007, 17008, 17009
- 1 mongos server at port 17000
- 1 stand alone instance at port 27017
All instances are currently running without user and password so for example, to connect to the **mongos** you can just use:
```
mongo mongodb://127.0.0.1:17001/admin
```
The sandbox can be started using the provided Makefile using: `make test-cluster` and it can be stopped using `make test-cluster-clean`

To run the unit tests, just run `make test`

### Build the exporter
The build process uses the dockerized version of goreleaser so you don't need to install Go.
Just run `make build` and the new binaries will be generated under the build directory.
```
├── build
│   ├── config.yaml
│   ├── mnogo_exporter_7c73946_checksums.txt
│   ├── mnogo_exporter-7c73946.darwin-amd64.tar.gz
│   ├── mnogo_exporter-7c73946.linux-amd64.tar.gz
│   ├── mnogo_exporter_darwin_amd64
│   │   └── mnogo_exporter   <--- MacOS binary
│   └── mnogo_exporter_linux_amd64
│       └── mnogo_exporter   <--- Linux binary
```
### Running the exporter
It you built the exporter using the method mentioned in the previous section, the generated binaries are in `mnogo_exporter_linux_amd64/mnogo_exporter` or `mnogo_exporter_darwin_amd64/mnogo_exporter` 

#### Example: running the exporter connecting to sandbox's primary
```
mnogo_exporter_linux_amd64/mnogo_exporter --mongodbdsn=mongodb://127.0.0.1:17001
```
#### Enabling collstats metrics gathering
`--mongodb.collstats-colls` receives a list of databases and collections to monitor using collstats. 
Usage example: `--mongodb.collstats-colls=database1.collection1,database2.collection2`

