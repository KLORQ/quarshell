# Project modules

Quarshell has 10 independent Go modules plus a TypeScript UI, Python SDK, documentation, and workspace repository. APIs remain unstable and use ascending `v0.x.y` versions. This table describes code boundaries; OpenShell parity gaps are tracked in the [compatibility register](../reference/openshell-compatibility.md).

| Module | Responsibility |
| --- | --- |
| [`quarshell-cli`](https://github.com/KLORQ/quarshell-cli) | `quarshell` CLI, user commands, local UX, and GUI helpers. |
| [`quarshell-core`](https://github.com/KLORQ/quarshell-core) | Domain types, credential primitives, shared ports, isolation contracts, and logging helpers. |
| [`quarshell-policy`](https://github.com/KLORQ/quarshell-policy) | Policy parser/evaluator, provider profiles, prover, and conformance logic. |
| [`quarshell-driver`](https://github.com/KLORQ/quarshell-driver) | Compute SPI, Docker/Podman/Kubernetes drivers, host isolation, and process supervisor. |
| [`quarshell-driver-vm`](https://github.com/KLORQ/quarshell-driver-vm) | MicroVM, worker, guest transport, KVM/HVF, and VM runtime orchestration. |
| [`quarshell-gateway`](https://github.com/KLORQ/quarshell-gateway) | Go backend: API/RPC, OIDC, orchestration, state, credential storage, and UI read models. |
| [`quarshell-runtime`](https://github.com/KLORQ/quarshell-runtime) | Guest init, supervisor, hardening, and sandbox image helpers. |
| [`quarshell-proxy`](https://github.com/KLORQ/quarshell-proxy) | Egress enforcement, credential mediation, and audit. |
| [`quarshell-sdk`](https://github.com/KLORQ/quarshell-sdk) | Public Go client for Gateway RPC. |
| [`quarshell-proto`](https://github.com/KLORQ/quarshell-proto) | Protobuf contracts and generated Go/Connect/gRPC/TypeScript bindings. |

## Other repositories

| Repository | Responsibility |
| --- | --- |
| [`quarshell-ui`](https://github.com/KLORQ/quarshell-ui) | TypeScript/React browser console; Gateway is the only Go backend. |
| [`quarshell-python`](https://github.com/KLORQ/quarshell-python) | Python SDK. |
| [`quarshell-deploy`](https://github.com/KLORQ/quarshell-deploy) | Helm and deployment assets. |
| [`sandbox.dev`](https://github.com/KLORQ/sandbox.dev) | MkDocs user and technical documentation. |
| [`quarshell-workspace`](https://github.com/KLORQ/quarshell-workspace) | Pins, workspace tooling, plans, and integration orchestration. |

Request path: CLI/UI/SDK → Gateway → compute driver → Docker/Podman or VM. The proxy enforces egress, policy supplies rules, and runtime manages processes inside the sandbox. UI calls the authenticated Gateway API only.
