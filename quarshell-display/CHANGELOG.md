# Changelog

## [Unreleased]

## [v0.1.6] - 2026-10-11

### Changed

- Rename the module, runtime identifiers and project references to the `cautem` namespace.

## [v0.1.0-beta.1] - 2026-10-10

### Changed

- Complete the cautem rebrand and pin the matching core source in CI.
- Align CI and module tooling with Go 1.27.2.
- Resolve `quarshell-core` from published v0.1.0-beta.2.

## [v0.1.0-alpha.2] - 2026-10-07

### Changed

- Use the published `quarshell-core` v0.1.0-alpha.2 dependency.
- Distribute the module under Apache-2.0.

## [v0.0.2-alpha.1] - 2026-09-28

### Security

- `OpenHostBrowser` accepts only absolute `http`/`https` URLs and passes them safely to the platform opener.
