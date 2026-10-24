<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# OpenShell compatibility

Quarshell's compatibility baseline is the OpenShell release pinned for this
work: [v0.1.4](https://github.com/NVIDIA/OpenShell/releases/tag/v0.1.4),
commit `5a7728c304b5528e954e9ae900cf21f84e1a12b1`. The source tag is checked out
under `tools/upstream/openshell-v0.1.4`; `UPSTREAM_VERSION` and `UPSTREAM_SHA`
are the baseline for generated schemas, the RPC inventory and the gap register.
This is a partial compatibility target, not a full parity claim.

The inventory contains 83 public `openshell.v1.OpenShell` RPCs, 12 compute
driver RPCs, five credential driver RPCs and related middleware/interceptor
services. Handler registration and proto descriptor checks establish API shape;
they do not establish matching behavior for every method or backend. The
release register records evidence and gaps by scenario.

The versioned feature matrix links each tracked feature to its backend, public
API or configuration surface, test references, evidence level and gate result.
`implemented`, `local_test`, `target_host_test` and `released` describe the
strongest evidence level; `gate_result` records pass, partial, fail or open.
The tracked feature inventory and upstream pin are validated by
`task compat:inventory` and `task compat:pin-check`.
No row claims `released` while the published-consumer gate is failing.

The Docker/Podman config inventory is generated from the pinned public driver
structs and checked against each local decoder. Its `value_behavior` records
omission, explicit empty/zero input and effective runtime behavior for all 54
fields: five rows are `verified` and 49 are `partial`, each with source/test
evidence. Partial rows name observed differences and remaining host/runtime
gates; they do not claim parity. Examples include Quarshell image/socket defaults,
Podman health-check lifecycle, proxy boolean presence and rootless ID mapping.
The config corpus remains a release blocker. The inventory also records
unsupported runtime-image settings and the intentional rejection of gateway
client certificates.


## Compatibility policy

Quarshell targets the latest stable OpenShell release pinned by tag and full
commit SHA. As of 2026-10-19, upstream marks v0.1.4 as the latest stable
release; moving `main`/`dev` builds are excluded. Full compatibility requires
behavioral parity across every listed API, CLI, config, provider, credential,
policy, backend, SDK, TUI and packaging capability—not just matching schemas.
The feature matrix below remains partial and release-blocking until every
required row has passing conformance evidence.

Quarshell does not support backward compatibility with earlier Quarshell or
Quarshell releases during this pre-beta cutover. The product will ship one
canonical `quarshell-*` module/package/config/environment/resource namespace.
No old-path imports, aliases, dual readers, fallback settings, old-resource
adoption or Quarshell-to-Quarshell migration adapter will be included. Existing
installations require a clean install; users must separately back up data they
need. The product will not silently delete unrelated old host resources.

## Compatibility status

| Surface | Current status |
|---|---|
| Wire schema | Generated from the pinned v0.1.4 source; drift and method counts are checked. Required upstream service and field names remain unchanged on the compatibility endpoint. |
| Public RPC handlers | Registered. A set of gaps still returns explicit unsupported/precondition results; per-method behavior is incomplete. |
| Docker and Podman | GitHub-hosted Linux lifecycle and egress acceptance pass for Docker and rootful/rootless Podman. Full 54-field config parity, raw-socket/DNS/Unix-socket host bypasses, outage recovery and clean published install remain open. |
| Logs and platform watch | Gateway invokes native Backend log and platform-event extensions. Remote compute plugins implement the same Backend contract; no Engine-to-Backend compatibility adapter remains. Streams are sandbox-scoped, while durable cursor and reconnect parity remain open. |
| Provider labels | Stored and returned; `ListProviders.label_selector` filters before pagination and rejects invalid selectors. |
| Operator credential export | `GetProviderCredentials` requires a verified mTLS peer certificate with the `operator` OU; bearer credentials are rejected for this method. Deployment and lifetime/refresh parity still need acceptance. |
| Stable placeholders and credential endpoint bindings | Not supported by the current runtime. Lint advertises support as false, and `GetSandboxProviderEnvironment` returns `CREDENTIALS_WITHHELD` instead of plaintext until the runtime can return policy-bound metadata. This is a release blocker. |
| MicroVM | Hosted Linux KVM lifecycle and nested Docker/rootful/rootless Podman acceptance pass; local Apple Silicon/HVF lifecycle and Gateway composition also pass. Full product Gateway-to-VM auth/activation, multi-Gateway rollout, the at-least-once cursor boundary, install/upgrade rollback and remaining credential/reporting cases still need feature-level acceptance. GPU/VFIO remains hardware-dependent and unsupported. |
| CLI and SDK | Top-level dispatch and command-specific help for every sandbox leaf are tested; help fixtures enumerate the accepted flags. The `sandbox create` parser also has acceptance, value-preservation, canonical-argv and malformed-input tests for every documented flag. Other nested parser behavior, output/timeout behavior, Python API and standalone/published consumers remain open. |

Provider readiness receipts, status queries and session-bound supervisor
observation reporting are implemented. MCP endpoint status reporting and its
durable per-sandbox snapshot are implemented. Sandbox-configuration admission
reporting is implemented without provisioning-deadline enforcement; several
peer RPCs and supervisor reporting paths remain unimplemented or partial.
The RPC inventory lists exact methods, stream direction, auth class, handler,
fixtures, tests and evidence. The gap register records remaining work:

- [Tracked OpenShell feature inventory](feature-inventory.yaml)
- [Remaining implementation and acceptance gaps](https://github.com/KLORQ/quarshell/blob/main/plans/REMAINING.md)
- [Pinned Docker/Podman config field inventory](https://github.com/KLORQ/quarshell/blob/main/quarshell-driver/internal/configcompat/testdata/openshell-v0.1.4-driver-config.json)
- [Runtime acceptance and test map](runtime-acceptance.md)
- [Architecture and backend SPI](../concepts/architecture.md)

## Recheck the pin

From the workspace root, run:

```sh
task compat:pin-check
task compat:proto
task compat:inventory
```

These checks confirm the checked-out source, generated proto drift, the RPC
inventory counts and the tracked gap baseline. They cannot discover a behavior
that has not yet been added to the inventory. Docker, Podman and MicroVM require
the separate host lanes documented in the [acceptance matrix](runtime-acceptance.md).

The canonical product module and package paths use the `quarshell-*` names.
Previous Quarshell paths are outside the supported beta contract; see the
[product and release contract](renaming-and-release.md).
