# k8s-apply-lib Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Changed
- [#11] Relicense to AGPL-3.0-only

## [v0.4.3] - 2024-09-18 - [RETRACTED RELEASE]
### Changed
- [#11] Relicense to AGPL-3.0-only

## [v0.4.2] - 2023-05-15
### Fixed
- [#9] Reduce technical debt

## [v0.4.1] - 2023-03-03
### Fixed
- [#7] Fix DoS vulnerability by upgrading the k8s controller-runtime

## [v0.4.0] - 2022-08-29
### Added
- [#5] Added general logging interface. See [Logger-Interface](apply/logger.go) for more information.

## [v0.3.0] - 2022-06-08
### Added
- [#3] Add function `WithApplyFilter` to support filtering resources before applying them.

## [v0.2.0] - 2022-06-07
### Added
- [#1] move apply functionality from k8s-ces-setup here