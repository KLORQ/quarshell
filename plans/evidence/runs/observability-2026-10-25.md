# Observability acceptance

- Verified: 2026-10-25 (Europe/Samara)
- Host: Darwin arm64, Go 1.27.3
- Scope: `OS-OBS` logs, audit events, traces, metrics, redaction and bounded buffering

## Evidence

- `go test -race ./... && go vet ./...` passed in `quarshell-gateway`.
- `go test -race ./... && go vet ./...` passed in `quarshell-runtime`.
- `go test -race ./slogx && go vet ./slogx` passed in `quarshell-core`.
- Supervisor exports traces and `quarshell.supervisor.workload.exits` to the same OTLP gRPC endpoint. An in-process collector test receives both signals, checks the stable service resource, W3C parent span ID and metric value. OTLP shutdown flushes both providers.
- Audit output is checked for an RFC3339Nano timestamp and OCSF class/activity/severity/action shape; empty values use stable schema defaults.
- slogx masking tests cover sensitive keys, inline secret detection, HTTP headers and gRPC metadata.
- Gateway log buffer tests cover per-sandbox line capacity, the shared retained-byte limit, bounded line/field payloads and cursor-gap reporting after eviction.

## Limits

This evidence verifies supervisor workload-exit metrics and does not claim a complete product-wide metric catalog. The exported metric uses only a bounded numeric exit-code attribute and contains no user, sandbox, token or credential values.

## Source digest

The digest is the SHA-256 of sorted per-file SHA-256 records for the supervisor OTLP setup/runtime/tests, audit event format/tests, gateway event and log buffers/tests, runtime logging setup, and `quarshell-core/slogx/*.go`.

`fe0687e95d69eb8ee1186f7f3af8b97268a5c01c16ce546c7500ae482943c56f`
