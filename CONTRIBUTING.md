# Contributing notes

## Local setup

The easiest way to make a local development setup is to use Docker Compose.

```
docker-compose up
make all testall
export MONGODB_URL='mongodb://localhost:27017'
./mongodb_exporter
```

`testall` make target will run integration tests. Set `TEST_MONGODB_URL` to run
against a non-localhost mongodb.


## Vendoring

We use [dep](https://github.com/golang/dep) to vendor dependencies.
Please use released version of dep (i.e. do not `go get` from `master` branch).
