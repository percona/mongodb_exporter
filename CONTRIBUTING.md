# Contributing notes

## Local setup

The easiest way to make a local development setup is to use Docker Compose.

```
docker-compose up
make all testall
export MONGODB_URL='mongodb://localhost:27017'
./mongodb_exporter
```

`testall` make target will run integration tests.


## Vendoring

We use [Glide](https://glide.sh) to vendor dependencies. Please use released version of Glide (i.e. do not `go get`
from `master` branch). Also please use `--strip-vendor` flag.
