# MongoDB exporter contributor guide
## Using the Makefile
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
|help|Display this help message.  |
|test|Run all tests (need to start the sandbox first)|
|test-cluster|Starts MongoDB test cluster. Use `env var TEST_MONGODB_IMAGE` to set flavor and version. Example:|
| |`TEST_MONGODB_IMAGE=mongo:3.6 make test-cluster`|
|test-cluster-clean|Stops MongoDB test cluster|

## Initializing the development environment  
First you need to have `Go` and `Docker` installed on your system and then, in order to install tools to format, test and build the exporter, you need to run this command:  
```  
make init  
```  
It will install `goimports`, `goreleaser`, `golangci-lint` and `reviewdog`.  
  
## Testing  
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

### Formating code
Before submitting code, please run `make format` to format the code according to the standards.
