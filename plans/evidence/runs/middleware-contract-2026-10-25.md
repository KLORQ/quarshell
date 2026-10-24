# OpenShell middleware contract acceptance

- Verified: 2026-10-25 (Europe/Samara)
- Host: Darwin arm64; Go 1.27.3
- Scope: ordered HTTP and WebSocket middleware stages and the typed gRPC extension contract

## Commands and result

- `go test -count=1 -race ./...` in `quarshell-proxy`: PASS.
- `go vet ./...` in `quarshell-proxy`: PASS.
- Focused tests exercise ordered pipeline mutation, typed request evaluation, response preflight/body/trailer exchange, WebSocket preflight and both directions, host selectors, unavailable endpoints, size bounds, fail-open/fail-closed behavior, cancellation and protected trailer mutation.
- A cancellation regression fixture blocks until the gRPC request context is cancelled and asserts the stage returns promptly with a denial under fail-closed policy.

## Source digest

SHA-256 over sorted per-file SHA-256 records for all Go source and tests under `quarshell-proxy/proxy/middleware` and `quarshell-proxy/internal/application/chain` (11 files):

`87c62301746efaa1554fbdb96f8714503d8ea9b9b61434cf7048b36f3010a1b2`

## Contract boundary

The proxy consumes external SupervisorMiddleware and HttpResponsePreReturn services. Their `Describe` and `ValidateConfig` RPC implementations belong to each external middleware provider; the proxy evaluates the configured request, response and WebSocket stages.
