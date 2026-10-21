# Agent instructions

## Scope

This repository contains provider profiles and effective-policy composition
for sandbox workloads.

## Working rules

- Treat provider credentials, endpoints, profile inheritance, and effective
  policy as security-sensitive. Avoid logging secret values.
- Keep profile examples and validation behavior aligned with the CLI, proxy,
  and provider profile guide in `quarshell-docs`.
- Keep local Go workspace replacements in the parent workspace's `go.work`,
  never in this repository's `go.mod`.

## Checks

- Run `go test ./...` from this checkout for Go changes.
- From the parent workspace, `task check` runs the shared Go gate.
