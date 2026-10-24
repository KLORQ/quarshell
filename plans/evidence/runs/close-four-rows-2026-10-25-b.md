# Four-row closure evidence: batch B

- Verified: 2026-10-25
- Host: Darwin 27.0.0 arm64, Go 1.27.3
- Scope: OS-CRED-API, OS-POLICY-LIFECYCLE, OS-CLI, OS-MAIN

## OS-CRED-API

- Backend: encrypted credential store; credential-driver RPCs served over gRPC `bufconn`.
- Command: `go test -count=1 -race ./credentialstore -run 'TestCredentialDriver'`
- Result: PASS. Capability negotiation, scoped opaque handles, create/overwrite/staged write, batched resolution, expiry, deletion, cross-workspace rejection, error redaction and RPC transport round trip are covered.
- Source digest: `dd035c2e3f3a1c45722b317a171a77891be5787c96ba102fc252c6f257b0e266` (sorted per-file SHA-256 records for `credentialstore/grpc_driver.go` and its tests).
- `ListCredentials` is optional in the pinned contract. The implementation advertises `SupportsList=false` and returns `UNIMPLEMENTED`.

## OS-POLICY-LIFECYCLE

- Backend: SQLite lifecycle store and OpenShell gRPC policy API.
- Command: `go test -count=1 -race ./internal/application/policy ./internal/transport/applicationgrpc -run 'TestPolicy|TestReportPolicyStatus|TestGlobalPolicyHistoryAndStatusSurviveSQLiteRestart'`
- Result: PASS. Draft/approve/reject, edit/undo/history, revision pagination, policy load ACK tied to active sandbox generation, and global workspace policy history/status survive SQLite reopen. Scope mixing is rejected.
- Source digest: `8b9350d9fece4f2ebae4e79efb2c4bc5837872d6c5b0e8d416e42c6ce3a52e4d` (sorted per-file records for policy application, SQLite lifecycle store, and policy gRPC handlers/tests).

## OS-CLI

- Backend: local CLI command tree and packages.
- Command: `go test -race ./... && go vet ./...` in `quarshell-cli`.
- Result: PASS across all CLI packages. Tests cover command parsing and scenarios, public/nested help inventory, leaf-specific help, completions, copy/upload, and terminal UI behavior.
- Source digest: `4358300c25e7f2d95ba85392028867f78c725e81977ce7ce5c77641e30bd59af` (sorted per-file records for all Go files under `quarshell-cli/`).

## OS-MAIN

- Backend: Gateway lifecycle store, supervisor process sessions, VM canonical process recorder.
- Commands:
  - `go test -count=1 -race ./internal/transport/applicationgrpc -run 'TestMainProcessExitHandlersPersistAndFinalizeActiveSupervisorResult|TestSandboxPhaseMapsCanonicalProcessExitStates|TestSignalMainProcessRPCUsesWorkspaceScopedCanonicalProcess'` in `quarshell-gateway`.
  - `go test -count=1 -race ./internal/app/supervisor ./supervisorcontrol ./relayclient` in `quarshell-runtime`.
  - `go test -count=1 -race ./vm -run 'TestRecordMainProcessExit'` in `quarshell-driver-vm`.
- Result: PASS. First terminal result is durable and generation-bound; duplicate/conflicting and stale reports are fenced; supervisor session and restart behavior passes. Live Apple Silicon/HVF lifecycle and Linux ARM64 guest process replay/reattach are recorded in [vm-main-output-replay-2026-10-24.md](vm-main-output-replay-2026-10-24.md).
- Source digest: `a28a61bd90af7c410233877a3da76e81377199ae7b7651feee70dbe4cf4c2bb3` (sorted per-file records for Gateway main-process handlers/tests, supervisor/session code/tests, VM process recording/tests, and retained guest process streams).
- Limits: upstream OpenShell supervisor-owned MainSession routing remains separate parity work; this row verifies the canonical runtime result/restart contract.

## Batch checks

- `go test -race ./... && go vet ./...` passed in `quarshell-gateway`.
- Focused race tests passed in `quarshell-runtime` and `quarshell-driver-vm`.
- `go test -race ./... && go vet ./...` passed in `quarshell-cli`.
- `task evidence:check` and `task evidence:report-check` passed after updating this matrix.
