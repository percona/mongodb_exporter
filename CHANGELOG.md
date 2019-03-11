# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/percona/mongodb_exporter/compare/v0.7.0...HEAD
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
