# Quarshell architecture consolidation

Updated: 2026-10-24.

The consolidation from 20 to 10 Go modules is complete. Product modules have
independent repositories and ascending pre-stable `v0.x` tags. The UI, Python
SDK, documentation site, and workspace tooling are separate repositories.
Root `main` is the default branch. The workspace tree still mirrors product
sources for cross-module CI; retiring that duplicate repository is a separate
cleanup step in the [current parity plan](REMAINING.md).

The previous version of this file mixed completed work with outdated CI logs,
module versions, and pending acceptance claims. Current OpenShell gaps and
acceptance requirements now live in [REMAINING.md](REMAINING.md). The pinned
source baseline and historical review are retained in
[`openshell-go-2026-10-21/`](openshell-go-2026-10-21/).

## Module boundaries

| Module | Responsibility |
|---|---|
| `quarshell-core` | Domain types, credential primitives and isolation contracts. |
| `quarshell-policy` | Policy parsing, validation, evaluation, profiles and prover. |
| `quarshell-proto` | Protocol schemas, generated bindings and OpenShell adapters. |
| `quarshell-driver` | Compute interface, Docker/Podman/Kubernetes and host isolation. |
| `quarshell-driver-vm` | MicroVM lifecycle, guest transport, KVM/HVF. |
| `quarshell-gateway` | Go backend, API, auth, orchestration and durable state. |
| `quarshell-runtime` | Guest init, supervisor, SSH and runtime hardening. |
| `quarshell-proxy` | Egress policy and credential mediation. |
| `quarshell-sdk` | Public Go Gateway client. |
| `quarshell-cli` | CLI, TUI and local UX. |

TypeScript UI, Python SDK, MkDocs site and workspace tooling are independent
non-Go repositories. The 10-module dependency graph is validated by
`task workspace:check` and `task dag:strict`.

## Completed consolidation

- Policy profiles and prover moved into `quarshell-policy`.
- Shared credentials, isolation ports and logging helpers were folded into
  `quarshell-core`; gateway persistence adapters remain in Gateway.
- Compute SPI, host supervisor and host isolation live in Driver; guest runtime
  remains in Runtime; VM worker and lifecycle are in Driver VM.
- Proto compatibility adapters live in Proto; Core does not depend on Proto.
- Workspace generation, dependency pins, module DAG, evidence and release tools
  are Go commands under `tools/cmd/quarshell-tools`.
- Old module skeletons that only contained boilerplate were removed; no runtime
  implementation was deleted as part of that cleanup.

## Current gates

The latest root Linux, macOS, Windows and hosted KVM workflows passed. The
standalone Driver v0.2.10 CI, Podman rootful/rootless and complete hosted KVM
engine acceptance passed. These are platform-level evidence; they do not close
all functional OpenShell parity rows. Follow the prioritized work in
[`REMAINING.md`](REMAINING.md).

Useful local gates:

```sh
task workspace:check
task ci:pins
task test
task compat:inventory
task evidence:check
task evidence:report-check
```
