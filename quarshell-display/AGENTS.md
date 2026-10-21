# Agent instructions

## Scope

This repository contains browser display and noVNC helpers used by GUI-enabled
sandboxes. Container and lifecycle wiring lives in `quarshell-driver` and
`quarshell-runtime`.

## Working rules

- Keep noVNC access password protected and avoid exposing display credentials
  through logs or command output.
- Coordinate image and runtime contract changes with the driver and runtime
  repositories; keep the public image reference in `quarshell-docs` accurate.
- Keep local Go workspace replacements in the parent workspace's `go.work`,
  never in this repository's `go.mod`.

## Checks

- Run `go test ./...` from this checkout for Go changes.
- From the parent workspace, `task check` runs the shared Go gate. GUI image and
  browser checks may require container tooling and a display-capable environment.
