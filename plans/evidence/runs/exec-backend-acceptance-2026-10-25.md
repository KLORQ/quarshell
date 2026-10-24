# Exec and terminal backend acceptance

- Verified: 2026-10-25 (Europe/Samara)
- Host profiles: Darwin arm64; GitHub-hosted Linux amd64
- Scope: non-TTY exec and interactive terminal behavior on shipped Docker, Podman and VM backends, plus SSH supervisor process sessions

## Current tree checks

- `task test`: PASS across all 10 Go modules.
- `go test -race ./... && go vet ./...` in `quarshell-cli`: PASS.
- `go test -race ./... && go vet ./...` in `quarshell-runtime`: PASS.
- `go test -race ./... && go vet ./...` in `quarshell-driver-vm`: PASS.

The tests cover non-TTY stdin and EOF, separate stdout/stderr, exit status, PTY input/output and resize, signal forwarding, cancellation/process-group cleanup, output bounds, replay cursors, retained-session reattach, and SSH process sessions.

## Live backend evidence

- Docker and Podman live Linux lifecycle/network/exec acceptance is recorded in [hosted-linux-acceptance-2026-10-22.md](hosted-linux-acceptance-2026-10-22.md), including the latest successful GitHub-hosted run listed there. Docker interactive exec checks resize, input, EOF and output. Podman uses the same Docker-compatible Engine API terminal implementation; its live lane verifies lifecycle and non-TTY exec.
- Apple HVF and Linux KVM VM terminal/process acceptance, including guest PTY resize, input, signal, output replay and reattach, is recorded in [vm-hvf-local-2026-10-22.md](vm-hvf-local-2026-10-22.md) and [vm-main-output-replay-2026-10-24.md](vm-main-output-replay-2026-10-24.md).

## Source digest

SHA-256 over sorted per-file SHA-256 records for all Go source and tests under `quarshell-driver`, `quarshell-driver-vm`, and `quarshell-runtime` (412 files):

`40e47f8e4fc57a3e85936a1a2ff03f5de03f04d0fc19a928e0cbd7d34db10321`

## Limits

No Kubernetes compute driver is shipped in this tree; Kubernetes configuration/client helpers are not claimed as a supported execution backend. The VM live evidence is from the existing acceptance records and was not rerun as part of this verification batch.
