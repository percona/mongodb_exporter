# Contributing notes

## Local setup

The easiest way to make a local development setup is to use Docker Compose.

```
docker-compose up
make all testall
./mongodb_exporter
```

`testall` make target will run integration tests against MongoDB instance specified in
`TEST_MONGODB_URI` environment variable (defaults to `mongodb://localhost:27017`).


## Vendoring

We use [dep](https://github.com/golang/dep) to vendor dependencies.
Please use released version of dep (i.e. do not `go get` from `master` branch).
