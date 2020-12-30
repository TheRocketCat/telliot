# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

NOTE: As semantic versioning states all 0.y.z releases can contain breaking changes in API \(flags, grpc API, any backward compatibility\)

We use _breaking :warning:_ to mark changes that are not backward compatible \(relates only to v0.y.z releases.\)

## Unreleased

#### Changed
* [\#321](https://github.com/tellor-io/telliot/pull/321) Unified all configuration files. LoggingConfig and LogLevel now reside in the main config file.

#### Added
* [\#339](https://github.com/tellor-io/telliot/pull/339) Initial support for Prometheus metrics.
* [\#340](https://github.com/tellor-io/telliot/pull/340) Added manifest files to run it in k8s google cloud with Prometheus and Grafana monitoring.

#### Fixed


## [v5.3.0](https://github.com/tellor-io/telliot/releases/tag/v5.3.0) - 2020.12.21

#### Changed

* [\#317](https://github.com/tellor-io/telliot/pull/317) Removed nodeURL and private key from config file
* [\#318](https://github.com/tellor-io/telliot/pull/318) `indexes.json` file format migrated to JSONPath format.

#### Added

* [\#272](https://github.com/tellor-io/telliot/pull/272) Automated Docker images on every push to master and with every tagged release.

## [v5.2.0](https://github.com/tellor-io/telliot/releases/tag/v5.2.0) - 2020.11.12

* [\#254](https://github.com/tellor-io/telliot/pull/254)
  * Added support for expanding variables in the indexer api url.
  * Added config to specify the `.env` file location. The default is now `configs/.env` so users should either specify a custom location in the `config.json` or move it inside the default config folder.

## [v5.0.0](https://github.com/tellor-io/telliot/releases/tag/v5.0.0) - 2020.11.02

#### Added

* Profitability calculations which is set through the `ProfitThreshold`\(in percents\) settings in the config,
* Docs how to contribute.
