# Evidence status

Generated from `matrix.yaml` by `tools/cmd/quarshell-tools evidence-report`.

- Baseline: OpenShell v0.1.4
- Upstream SHA: `5a7728c304b5528e954e9ae900cf21f84e1a12b1`
- Rows: 29 (required 29)
- Promotion: NOT READY
- Counts: pass=15, unverified=14

| ID | Owner | Required | Result | Level | Backend | Host | Artifact |
| --- | --- | --- | --- | --- | --- | --- | --- |
| OS-API | gateway + proto | yes | unverified | - | - | - | - |
| OS-COMPUTE | driver + proto | yes | unverified | - | - | - | - |
| OS-CRED-API | credentials | yes | pass | integration | encrypted credential store over gRPC bufconn | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25-b.md |
| OS-MIDDLEWARE | proxy + extensions | yes | pass | integration | gRPC middleware stages and ordered proxy pipeline | Darwin arm64, Go 1.27.3 | plans/evidence/runs/middleware-contract-2026-10-25.md |
| OS-INTERCEPTORS | gateway + extensions | yes | unverified | - | - | - | - |
| OS-ISOLATION-PROTO | isolation + supervisor | yes | unverified | - | - | - | - |
| OS-GATEWAY-DEPLOY | gateway + CLI | yes | unverified | - | - | - | - |
| OS-SANDBOX | gateway + driver | yes | pass | integration | SQLite durable store + fake runtime driver | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25.md |
| OS-MAIN | supervisor + isolation | yes | pass | integration | Gateway lifecycle store + supervisor process session + VM main-process recorder | Darwin arm64, Go 1.27.3; Linux ARM64 guest acceptance | plans/evidence/runs/close-four-rows-2026-10-25-b.md |
| OS-EXEC | supervisor + SDK | yes | pass | integration | Docker-compatible Engine API (Docker/Podman), VM guest retained process, SSH supervisor | Darwin arm64 and GitHub-hosted Linux amd64; Go 1.27.3 | plans/evidence/runs/exec-backend-acceptance-2026-10-25.md |
| OS-WORKSPACE | gateway | yes | pass | integration | SQLite/memory workspace lifecycle; Gateway sandbox/template/service registries | Darwin arm64, Go 1.27.3 | plans/evidence/runs/workspace-scope-2026-10-25.md |
| OS-POLICY-SCHEMA | policy | yes | pass | integration | quarshell-policy + pinned OpenShell v0.1.4 oracle | Darwin arm64; Go 1.27.2; Rust 1.98.0 | plans/evidence/runs/policy-differential-2026-10-22.md |
| OS-POLICY-LIFECYCLE | gateway + supervisor | yes | pass | integration | SQLite lifecycle store + OpenShell gRPC policy API | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25-b.md |
| OS-FS | isolation | yes | unverified | - | - | - | - |
| OS-NET | isolation + proxy | yes | unverified | - | - | - | - |
| OS-L7 | proxy + policy | yes | pass | integration | Quarshell policy-aware network proxy and protocol adapters | Darwin arm64, Go 1.27.3 | plans/evidence/runs/l7-protocol-acceptance-2026-10-25.md |
| OS-CREDENTIALS | credentials + supervisor + proxy | yes | unverified | - | - | - | - |
| OS-TOKEN | credentials | yes | unverified | - | - | - | - |
| OS-PROVIDERS | providers | yes | pass | unit | pinned OpenShell v0.1.4 provider fixtures + bundled catalog | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25.md |
| OS-BACKENDS | driver / driver-vm / driver-mxc | yes | unverified | - | - | - | - |
| OS-GPU | compute + vm | yes | unverified | - | - | - | - |
| OS-CRED-DRIVERS | credentials | yes | pass | integration | PostgreSQL 16 + Kubernetes fake client + Vault-compatible httptest server | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25.md |
| OS-PEER | gateway | yes | unverified | - | - | - | - |
| OS-PROVER | prover | yes | pass | integration | Z3 5.3.1 | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25.md |
| OS-OBS | telemetry + владельцы событий | yes | pass | integration | OTLP gRPC traces and metrics; bounded gateway observation log ring; OCSF audit events; slogx redaction | Darwin arm64, Go 1.27.3 | plans/evidence/runs/observability-2026-10-25.md |
| OS-CLI | CLI | yes | pass | unit | Quarshell CLI command tree and local test suite | Darwin arm64, Go 1.27.3 | plans/evidence/runs/close-four-rows-2026-10-25-b.md |
| OS-SDK | SDK + client packages | yes | pass | integration | Live Go Gateway process with HTTP and gRPC listeners | Darwin arm64, Go 1.27.3, Node 22, Python 3.13 | plans/evidence/runs/sdk-gateway-e2e-2026-10-25.md |
| OS-DISTRIBUTION | release tooling | yes | unverified | - | - | - | - |
| OS-DOCS | docs + CLI | yes | unverified | - | - | - | - |
