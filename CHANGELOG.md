# Changelog

## v0.5.0 (not released yet)

* Removed tests for EOL'ed MongoDB 3.0 and Percona Server for MongoDB 3.0.

## v0.4.0 (2018-01-17)

* New flags `-collect.database` and `-collect.collection` can be used to enable collection of database and collection
  metrics. They are disabled by default.
* MongoDB connections are now kept between the scrapes. New flag `-mongodb.max-connections` (with the default value `1`)
  controls the maximum number of established connections.
* Add standard metrics:
  * `mongodb_scrape_errors_total`
  * `mongodb_up`
* Some queries now contain [cursor comments](https://www.percona.com/blog/2017/06/21/tracing-mongodb-queries-to-code-with-cursor-comments/)
  with source code locations.
* Go vendoring switched to [dep](https://github.com/golang/dep).

## v0.3.1 (2017-09-08)

* Better logging for scrape errors.

## v0.3.0 (2017-07-07)

* Add standard metrics:
  * `mongodb_exporter_scrapes_total`
  * `mongodb_exporter_last_scrape_error`
  * `mongodb_exporter_last_scrape_duration_seconds`
* Fix a few data races.

## v0.2.0 (2017-06-28)

* Default listen port changed to 9216.
* All log messages now go to stderr. Logging flags changed.
* Fewer messages on default INFO logging level.
* Use https://github.com/prometheus/common log for logging instead of https://github.com/golang/glog.
* Use https://github.com/prometheus/common version to build with version information.
* Use https://github.com/prometheus/promu for building.

## v0.1.0

* First tagged version.
