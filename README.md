# Quarshell module workspace

This repository contains Quarshell's independently versioned Go modules and
thin Python/TypeScript clients. Run Git commands from the repository root.
Each Go module keeps its own `go.mod`; the root `go.work` is the local
multi-module development workspace.

Product documentation is at [sandbox.dev](https://klorq.github.io/sandbox.dev/). Each module README covers its own API and local checks. The [OpenShell compatibility status](https://klorq.github.io/sandbox.dev/reference/openshell-compatibility/) states what has and has not been verified.

## Active plan

The workspace is being rebuilt as a modular Go implementation of OpenShell
v0.1.4. The adopted plan, review, baseline snapshot and live progress checklist
are in [`plans/`](plans/README.md).

Start with [`plans/CHECKLIST.md`](plans/CHECKLIST.md): it records every addition
to the project and the status of M0–M8.

## Modules

| Module | Purpose |
| --- | --- |
| `quarshell-cli` | CLI and agent images |
| `quarshell-core` | Shared domain values and errors |
| `quarshell-policy` | Policy schema, validation and evaluation |
| `quarshell-spi` | Runtime and backend interfaces |
| `quarshell-credentials` | Credential broker and stores |
| `quarshell-telemetry` | Event and telemetry contracts |
| `quarshell-providers` | Provider profiles and composition |
| `quarshell-proxy` | Network policy and egress proxy |
| `quarshell-isolation` | Linux isolation and mediation |
| `quarshell-supervisor` | Sandbox process supervision |
| `quarshell-display` | Browser display support |
| `quarshell-driver` | Docker, Podman and Kubernetes backends |
| `quarshell-driver-vm` | VM compute backend |
| `quarshell-driver-mxc` | Windows MXC backend |
| `quarshell-gateway` | Control plane, registry and relay |
| `quarshell-proto` | Protobuf contracts and generated clients |
| `quarshell-prover` | Policy containment and risk analysis |
| `quarshell-runtime` | Runtime integration and process lifecycle |
| `quarshell-sdk` | Go client |
| `quarshell-slogx` | Shared process logging (MIT) |
| `quarshell-python` | Python client |
| `quarshell-ui` | TypeScript client and UI bindings |
| `quarshell-docs` | Source for the public site |
| `quarshell-org-profile` | GitHub organization profile |

The repository root contains workspace/build configuration, CI, deployment,
and migration acceptance materials required to build and verify these modules.
Generated output, local credentials and private keys are ignored.

The root `go.work` links the Go modules. Regenerate it after a module-path change with `task workspace:sync`, then check it with `task workspace:check`.

## Local checks

```bash
task check
task compat:pin-check
task compat:gaps
task docs:strict
```

All product modules use Apache-2.0 except `quarshell-slogx`, which remains MIT.
