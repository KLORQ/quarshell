# OpenShell L7 protocol acceptance

- Verified: 2026-10-25 (Europe/Samara)
- Host: Darwin arm64; Go 1.27.3
- Scope: policy-aware proxy protocol parsing, forwarding, allow/deny, size limits and malformed-input handling

## Commands and result

- `go test -count=1 -race ./...` in `quarshell-proxy`: PASS.
- `go vet ./...` in `quarshell-proxy`: PASS.
- The suite includes HTTP request/response and TLS termination, TCP CONNECT decisions, WebSocket handshakes and bidirectional frames, JSON-RPC batches and malformed messages, MCP typed messages and SSE, GraphQL parsing/batches/WebSocket operation rules, and size and policy update behavior.
- SQL audit mode passes TLS record bytes through unchanged. This matches the pinned OpenShell contract, which does not parse SQL; it is not SQL statement inspection.

## Source digest

SHA-256 over sorted per-file SHA-256 records for all Go source and tests under `quarshell-proxy/proxy`, `quarshell-proxy/internal/application/httpproxy`, `quarshell-proxy/internal/application/mediation`, and `quarshell-proxy/internal/application/chain` (77 files):

`53ba923375929de5c2e0fce3a2b95a1f0ee54f4a62a67b976e95dcf33758a0f2`
