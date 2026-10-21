# Agent instructions

## Scope

This repository provides structured logging helpers built on Go's
`log/slog`. Its published module path remains `github.com/KLORQ/quarshell-slogx` and its
license is MIT.

## Working rules

- Preserve the documented logging API and avoid exposing secrets or sensitive
  payload fields. Review masking behavior for nested and structured values.
- Keep examples, README, and changelog in sync with exported API changes.
- Do not change the published module path or license as part of routine work.

## Checks

- Run `go test ./...` from this checkout for Go changes.
- Run the Makefile's lint and release checks before publishing a release.
