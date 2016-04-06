# Mongodb Exporter

Based on  MongoDB exporter for prometheus.io, written in go (https://github.com/dcu/mongodb_exporter), but forked for full sharded support and  structure changes.

## Building

    export GO_VERSION=1.5.1  # if you wish to use your system version
    make


## Note about how this works
Point the process to any mongo port and it will detect if it is a mongos, replicaset member, or stand alone mongod and return the appropriate metrics for that type of node. This was done to preent the need to an exporter per type of process.

## Roadmap

- Document more configurations options here


