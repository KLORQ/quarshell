# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed

- Remove the non-OpenShell HTTP log-level control handler and listener.

## [v0.1.0-beta.1] - 2026-10-10

### Changed

- Complete the cautem rebrand while retaining the published `github.com/KLORQ/quarshell-slogx` Go module path.
- Pin the Go 1.27.2 toolchain used by the release checks.

## [v0.1.0-alpha.2] - 2026-10-07

### Changed

- Publish the corrected `github.com/KLORQ/quarshell-slogx` module path; the previous alpha tag declared an older path.
- Update OpenTelemetry modules from v1.46.0 to v1.47.0.
- Name the environment polling interval; simplify map copying without changing the public API.

## [v0.1.0-alpha.1]

### Added

- Go 1.27 module baseline
- Corporate masking: `CorporateMasker`, `CorporateMaskRules`, `WithCorporateMasking`, JWT/AWS/PEM/IBAN/Luhn auto-detect, fingerprints
- Live level control: `ParseLevel`, `SetLevelString`, `WatchLevelEnv`
- Context recovery of slogx config from `slog.Default()` when installed via `SetupDefault`
- `Corporate()` preset alias; production presets enable corporate masking by default

### Changed

- Mask key matching is case/separator-insensitive with nested and suffix lookup
- Email/phone/card redaction shapes tightened for corporate logs

## [v0.0.2-alpha.1] - 2026-09-28

### Fixed

- Token masking no longer emits stable fingerprints by default; `WithCorporateMasker(true)` enables them explicitly.
- `RedactAuditText` removes URL query values and common inline credentials from audit details.
