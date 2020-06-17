# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.11.0]
### Changed
- `go.mongodb.org/mongo-driver` was updated to `v1.3.2`.
- `github.com/prometheus/client_golang` was updated to `v1.5.1`.
- [PMM-4719](https://jira.percona.com/browse/PMM-4719): Remove redundant flags from "mongodb_exporter" if possible. 
Those flags have been removed: `--mongodb.authentification-database, --mongodb.max-connections, --mongodb.socket-timeout, --mongodb.sync-timeout`. You can use [connection-string-options](https://docs.mongodb.com/manual/reference/connection-string/#connection-string-options) instead.
- Added lost connection metrics and removed useless file [@nikita-b](https://github.com/nikita-b)
### Added

### Fixed
- [PMM-2717](https://jira.percona.com/browse/PMM-2717): Failed to execute find query on 'config.locks': not found. source="sharding_status.go:106". 
All `mongodb_mongos_sharding_balancer_lock_*` metrics won't be exposed for `MongoDB 3.6+`. See: https://docs.mongodb.com/v3.6/reference/config-database/#config.locks.

## [0.10.0]
### Changed
- `go.mongodb.org/mongo-driver` was updated to `v1.1.1`.
- All `--mongodb.tls*` flags were removed. Use [tls-options](https://docs.mongodb.com/manual/reference/connection-string/#tls-options) instead.

### Added

### Fixed

## [0.9.0]
### Changed

### Added
- [PMM-4131](https://jira.percona.com/browse/PMM-4131): Added missing features from [dcu/mongodb_exporter](https://github.com/dcu/mongodb_exporter). See list below.
- New metrics:
  - `mongodb_mongod_replset_member_*`
  - `mongodb_connpoolstats_*`
  - `mongodb_tcmalloc_*`
- Added application name "mongodb_exporter" to mongo logs, [@nikita-b](https://github.com/nikita-b)

### Fixed
- [PMM-4427](https://jira.percona.com/browse/PMM-4427): Panic when read rocksdb status, txh [@lijinglin2019](https://github.com/lijinglin2019).
- [PMM-4583](https://jira.percona.com/browse/PMM-4583): Fix panic when GetTotalChunksByShard (#158), txh [@lijinglin2019](https://github.com/lijinglin2019).

## [0.8.0]
### Changed
- [PMM-3511](https://jira.percona.com/browse/PMM-3511): Switched to [mongo-go-driver](https://github.com/mongodb/mongo-go-driver).
- [PMM-4168](https://jira.percona.com/browse/PMM-4168): Updated all dependencies including `github.com/prometheus/golang_client`.
- Moved `opcounters`, `opcountersRepl` to `common_collector` (#146), thx [@nikita-b](https://github.com/nikita-b).

### Added

### Fixed

## [0.7.1]
### Added
- Added Authentification Database option when connect to mongo #139, thx [@etiennecoutaud](https://github.com/etiennecoutaud).
- Added helm chart to readme #140, thx [@pgdagenais](https://github.com/pgdagenais).
- [PMM-4154](https://jira.percona.com/browse/PMM-4154): Added standard logging flags.

### Fixed
- Fixed some function comments based on best practices from Effective Go #137, thx [@CodeLingoBot](https://github.com/CodeLingoBot).
- [PMM-3473](https://jira.percona.com/browse/PMM-3473): Fixed panic and runtime error.

## [0.7.0]
### Changed
- [PMM-3512](https://jira.percona.com/browse/PMM-3512): Switched to [kingpin](https://github.com/alecthomas/kingpin) library.
This is a **BREAKING CHANGE** because kingpin uses `--` instead of `-` for long flags, so be careful when updating.
- [PMM-2261](https://jira.percona.com/browse/PMM-2261) Unify common mongod and mongos server metrics, thx [@bz2](https://github.com/bz2)
This is a **BREAKING CHANGE**. The labels of these metrics are now prefixed with just `mongodb_` rather than `mongodb_mongo[ds]_`.

### Added
- Fine grained error handling for index usage and collection stats (#128), thx [@akira-kurogane](https://github.com/akira-kurogane)
- Introduce a docker go build that creates a mongodb_exporter binary within a container (#112), thx [@mminks](https://github.com/mminks)
- Ability to make releases and snapshots with [GoReleaser](https://goreleaser.com/)

## [0.6.3] - 2019-02-13
### Added
- PMM-3401: Added collection of TTL metrics #127, thx [@fastest963](https://github.com/fastest963)
- Added some new metrics:
  - `member_replication_lag`
  - `member_operational_lag`
  - `op_latencies_latency_total`
  - `op_latencies_ops_total`
  - `op_latencies_histogram`

### Fixed
- Fix for broken labels on `index_usage` metrics
- Fix SIGSEGV when running connected to a Primary, thx [@anas-aso](https://github.com/anas-aso)

### Changed
- Move CHANGELOG.md to "Keep a Changelog" format.

## [0.6.2] - 2018-09-11
### Added
- Build binaries #110
- Test `--help` flag for diff of options between releases #111

## [0.6.1] - 2018-06-15
### Fixed
- `--version` now properly reports `0.6.1`

## [0.6.0] - 2018-06-13
### Added
- Add required testify dependency #107, thx [@RubenHoms](https://github.com/RubenHoms)
- Add timeout flags #100, thx [@unguiculus](https://github.com/unguiculus)
- Enable collection of table top metrics #94, thx [@bobera](https://github.com/bobera)
- Individual index usage stats and index sizes added #97, thx [@martinhoefling](https://github.com/martinhoefling)
- Support `&ssl=true` #105 #90, thx [@dbmurphy](https://github.com/dbmurphy)

### Fixed
- Fix `balancerIsEnabled` & `balancerChunksBalanced` values #106, thx [@jmsantorum](https://github.com/jmsantorum)

## [0.5.0] - 2018-05-24
### Added
- Check connection with exporter #92. Adds `--test` flag to verify connection with MongoDB and quits.

### Removed
- Removed tests for EOL'ed MongoDB 3.0 and Percona Server for MongoDB 3.0.

### Fixed
- Redact Mongo URI #101. Fixes URI logging in plain text including credentials when no session can be created.
- ARM64-specific fixes #102. Fixed two portability issues in Makefile.

## [0.4.0] - 2018-01-17
### Added
- New flags `-collect.database` and `-collect.collection` can be used to enable collection of database and collection
  metrics. They are disabled by default.
- MongoDB connections are now kept between the scrapes. New flag `-mongodb.max-connections` (with the default value `1`)
controls the maximum number of established connections.
- Add standard metrics:
  - `mongodb_scrape_errors_total`
  - `mongodb_up`
- Some queries now contain [cursor comments](https://www.percona.com/blog/2017/06/21/tracing-mongodb-queries-to-code-with-cursor-comments/)
with source code locations.

### Changed
- Go vendoring switched to [dep](https://github.com/golang/dep).

## [0.3.1] - 2017-09-08
### Changed
- Better logging for scrape errors.

## [0.3.0] - 2017-07-07
### Added
- Add standard metrics:
  - `mongodb_exporter_scrapes_total`
  - `mongodb_exporter_last_scrape_error`
  - `mongodb_exporter_last_scrape_duration_seconds`

### Fixed
- Fix a few data races.

## [0.2.0] - 2017-06-28
### Changed
- Default listen port changed to 9216.
- All log messages now go to stderr. Logging flags changed.
- Fewer messages on default INFO logging level.
- Use https://github.com/prometheus/common log for logging instead of https://github.com/golang/glog.
- Use https://github.com/prometheus/common version to build with version information.
- Use https://github.com/prometheus/promu for building.

## [0.1.0] - 2016-04-13
### Added
- First tagged version.

[Unreleased]: https://github.com/percona/mongodb_exporter/compare/v0.10.0...HEAD
[0.10.0]: https://github.com/percona/mongodb_exporter/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/percona/mongodb_exporter/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/percona/mongodb_exporter/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/percona/mongodb_exporter/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/percona/mongodb_exporter/compare/v0.6.3...v0.7.0
[0.6.3]: https://github.com/percona/mongodb_exporter/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/percona/mongodb_exporter/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/percona/mongodb_exporter/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/percona/mongodb_exporter/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/percona/mongodb_exporter/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/percona/mongodb_exporter/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/percona/mongodb_exporter/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/percona/mongodb_exporter/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/percona/mongodb_exporter/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/percona/mongodb_exporter/compare/14803c0f7aed483297a06b3fcfacafee5cf1b8f9...v0.1.0
