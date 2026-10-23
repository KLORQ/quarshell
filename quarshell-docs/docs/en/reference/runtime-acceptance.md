<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Runtime acceptance and test map

> Historical evidence note: PR numbers, commit SHAs, and published module
> versions cited below describe the repository history and artifacts before
> the clean-history reset. They are retained as audit context and do not prove
> that the current source checkouts passed those lanes or were released.

This matrix tracks the behavior corpus from OpenShell v0.1.4 and the backend
checks required before Quarshell can call a runtime production-ready. It is an
engineering gate, not a compatibility claim.

## Status meanings

| Status | Meaning |
|---|---|
| `local tests` | Unit, protocol, or integration tests exist and pass in the current checkout. |
| `host gate open` | Real engine, daemon recovery, security boundary, or published artifact still needs acceptance on its target host. |
| `unsupported` | The request is rejected or withheld explicitly; there is no silent downgrade. |
| `open` | Implementation or behavior evidence is incomplete. |

## API and user-visible behavior corpus

| Feature group | Coverage in this checkout | Test / evidence | Remaining gate |
|---|---|---|---|
| Public control protocol | Inventory is pinned to v0.1.4: 83 public RPCs, plus related compute, credential, middleware and interceptor services. Handler presence is shape coverage only. | [tracked feature inventory](https://github.com/KLORQ/sandbox.dev/blob/main/docs/ru/reference/feature-inventory.yaml) | Per-method golden inputs/results, stream cancellation, metadata and error parity remain open. |
| Sandbox create and inspect | Gateway maps the pinned OpenShell create request once to `BackendCreateRequest`; preflight, create, lifecycle, rollback and recovery call registered modular `driver.Backend` implementations directly. VM exec, process/log/transfer, policy and platform watch use declared native extensions. No flat `Spec` or `Engine` adapter remains. | `rpc_sandbox_test.go`, `rpc_templates_test.go`, `runtime_recovery_test.go`, `rpc_exec_test.go`, `remote_compute_backend_test.go`, `rpc_watch_test.go`; Driver backend contract and capability tests. | Full pinned CLI/backend per-method behavior corpus remains open; stream cancellation/event resume parity needs further coverage. VM Gateway create remains fail-closed until the managed host supervisor and authenticated host/guest isolation contract are complete. |
| OCI image inputs | MicroVM accepts a registry reference or the staged rootfs tar path. Registry tags are resolved to manifest digests; the VM selects `linux/<host-arch>`, checks image config and compressed layer digests/sizes, applies whiteouts, and builds an immutable ext4 base. Cache identity includes source and selected manifest digests, platform, rootfs layout version and guest-agent digest. A 64 GiB default cache quota evicts only images without durable sandbox references or active create leases. Docker/Podman keychain credentials are used for private registries; local acceptance exercises a Docker credential helper. OCI `Entrypoint`, `Cmd`, `Env`, `User` and `WorkingDir` feed the in-memory launch spec; recovery fetches config by the selected immutable platform manifest digest. Image env/user are not persisted, reserved identity variables are filtered, and root identities fail closed. Linux workers start in private user/mount namespaces, make mount propagation private and receive no inherited Gateway environment; namespace root maps to the Gateway host account and is not a dedicated host UID. | `quarshell-driver-vm/vm/oci_image_test.go`, `quarshell-driver-vm/vm/launch_spec_test.go`, `quarshell-driver-vm/vm/live_acceptance_test.go`, `quarshell-driver-vm/vmguest/agent_linux_test.go`, `image_cache_test.go`, `rootfs_image_test.go`, `rootfs_path_test.go`, `worker_process_linux_test.go`, `vm_test.go` | Additional credential-helper implementations and TLS/auth variants on target hosts; dedicated host UID, fd-based mount allowlist, UID/GID mapping and persistence; Gateway/Linux KVM acceptance. |
| Lifecycle, restart and host↔guest protocol | Docker/Podman lifecycle, recovery and main-process restart policies have package and gateway coverage. VM control frames use protocol v1; both sides reject missing or unknown versions, and guest responses carry the supported version. `start-workload` waits for a private supervisor readiness marker written only after the supervisor starts the workload; the marker path is removed from the workload environment. Start and worker recovery rotate the per-sandbox token through a durable pending record; replay with the old token is accepted only for the matching candidate and rotation ID. The guest forwards the validated restart policy to its supervisor; macOS ARM64 live acceptance observed an `on-failure` restart with the current host and guest agent. A fresh VM driver can hydrate the launch spec of an already-running sandbox before adopting its worker, without persisting image environment or identity. | `quarshell-driver-vm/vm/control_test.go`, `quarshell-driver-vm/vm/state_test.go`, `quarshell-driver-vm/vm/lifecycle_test.go`, `quarshell-driver-vm/vmguest/agent_linux_test.go`, `quarshell-runtime/internal/app/supervisor/run_test_unix_test.go`, `quarshell-driver/tests/e2e`, `quarshell-gateway/tests/e2e`, `rpc_main_process_test.go`, `task vm:live-darwin` | Protocol upgrade negotiation and versioned service/worker control contracts; live daemon/Gateway outage and foreground-attach corpus. Gateway VM recovery remains open. |
| Exec, attach and TTY | VM exposes a separate full-duplex `ExecTerminal` SPI extension. The authenticated guest PTY streams stdin/stdout, applies window resize, delivers terminal control characters such as Ctrl-C, and reports normalized exit status. VM sessions expose a stable ID, reattach to the same guest process after the control stream drops, and terminate it by session ID while detached; detached output has a bounded replay tail. Gateway stores the VM session ID with the durable exec request hash and a retry of the same TTY `request_id` reattaches without relaunching. Canonical VM workloads with `TTY=true` retain a boot-scoped main-process PTY in the guest and reattach to the same PID after driver recreation; local HVF acceptance verifies input/output, 101×41 resize and process identity. Foreground TTY creates pass the command and `AwaitMainProcessAttachment` through the durable create record. The CLI attaches to that retained PTY through the additive Quarshell `MainProcessService` gRPC extension; Gateway returns the canonical process exit after the stream closes. HVF acceptance verifies that a short TTY workload retains output and exit until a late attachment closes. | `quarshell-driver-vm/vm/terminal.go`, `quarshell-driver-vm/vmguest/retained_terminal.go`, `quarshell-driver-vm/vm/live_acceptance_test.go::assertLiveMainProcessTTY`, `quarshell-driver-vm/vm/live_acceptance_test.go::TestLiveMicroVMLifecycle`, `quarshell-cli/internal/service/sandbox_gateway_test.go::TestRemoteSandboxCreateForegroundAttachesToCanonicalProcess`, `quarshell-sdk/go/klorq/exec_interactive.go::AttachMainProcessTTY`, `quarshell-gateway/internal/transport/applicationgrpc/main_process_test.go::TestAttachTTYStreamsCanonicalMainProcessAndExit`; `task vm:live-darwin` | Main-process attachment behavior for workloads without a TTY and the host-side `RunningBoundary` process handle remain incomplete. Hosted Linux/KVM acceptance and live interactive exec on Docker/Podman hosts remain open. |
| Logs, cursor and watch | Gateway log and event streaming are tested. | `rpc_logs_test.go`, `rpc_watch_test.go` | Loss-aware resume, cursor expiry and reconnect parity against v0.1.4 fixtures. |
| Copy and upload backpressure | Driver and SDK copy paths have tests; rootfs staging uses the v0.1.4 workspace-scope request. | Driver transfer tests; `rootfs_tar_test.go`; `rpc_rootfs_test.go` | Large upload backpressure, cancellation and bounded-memory acceptance on live engines. |
| Service forward and SSH | Gateway service and relay flows have tests; public SSH sessions are expiring and sandbox-scoped. SSH/TCP/peer relays and supervisor exit/finalize calls validate the shared durable owner epoch; active streams close within a 250 ms polling window after handoff to fence a stale Gateway replica. | `rpc_services_test.go`, `rpc_ssh_test.go`, `tests/e2e/engine_connect_test.go`, `runtime_relay_test.go::TestRelayStreamRejectsPendingChannelFromStaleGatewayOwner`, `runtime_relay_test.go::TestActiveSupervisorRelayIsClosedAfterDurableOwnerHandoff`, `supervisor_lease_test.go::TestSupervisorLeaseRemainsValidUntilNonzeroMainExitIsFinalized` | Upstream host-supervisor process bootstrap and full attach/session replay semantics; published-binary smoke and daemon-restart acceptance per backend. |
| Provider CRUD and labels | Provider labels are persisted, returned and filtered before pagination. Selector validation follows the pinned equality-selector syntax. | `rpc_providers_test.go`, `store/clone_test.go` | Paging-token binding and full imported-provider fixtures remain open. |
| Provider profiles and refresh | Profile parsing, credential refresh, external credential drivers, token exchange and secret redaction have local tests. | `rpc_profiles_test.go`, `rpc_credentials_test.go`, `driver_registry_test.go` | Per-profile expected defaults and runtime effects remain open. |
| Operator credential export | v0.1.4 handler requires a verified mTLS peer with the `operator` certificate OU and rejects bearer auth. | `provider_credentials_export_test.go`, `grpc_auth_test.go` | Deployment with operator certificates, refresh/lifetime parity and audit review. |
| Endpoint-bound credentials | `GetSandboxProviderEnvironment` returns `CREDENTIALS_WITHHELD` until endpoint bindings are produced. Lint reports stable-placeholder support as false. | `rpc_credentials_test.go`, `rpc_profiles_test.go` | Implement policy-bound bindings, credential identity, stable handles, refresh and supervisor installation together. This is a release blocker. |
| Policy and egress | Policy validation, updates, active proxy decisions and credential placeholder rewriting have local tests. | `rpc_policy_*_test.go`, `quarshell-proxy/proxy/*_test.go` | MicroVM live test reloads revision 2, observes broker acknowledgement and verifies allow/deny decisions. Docker/Podman target-host dual-stack, Gateway-to-VM live verification, Linux KVM and Podman 6 route gates remain open. |
| Workspace, RBAC and quotas | Workspace scoping and authorization paths have gateway tests. | `rpc_workspaces_test.go`, `workspace_quota_test.go` | Complete auth-method and pagination corpus against the pinned CLI/SDK. |
| CLI and SDK parity | Go SDK methods and command dispatch build; standalone consumers have a dedicated test target. Gateway MicroVM exec now uses the pinned login shell by default and `-c` for `--no-login-shell`; guest tests cover quoting/profile behavior and the macOS ARM64 live lane verifies Alpine `.profile` behavior. | `quarshell-sdk/tests/consumer`; CLI module tests; `quarshell-driver-vm/vmguest/agent_linux_test.go`; `task vm:live-darwin` | Leaf commands, flags, output, timeout process termination, Python client and published dependency resolution. |

## Backend gates

| Capability | Docker | Podman | MicroVM |
|---|---|---|---|
| Create, inspect, start, stop, delete | Docker-in-Docker E2E passed locally; supported host matrix remains open | Disposable Podman 5.8.7 lifecycle E2E passed; rootful/rootless target-host gate remains open | Driver-level live acceptance passed on macOS ARM64, including restart; Gateway create remains fail-closed pending host supervisor/isolation |
| CPU and memory limits | E2E resource admission passed; target daemon limits need host matrix | Resource behavior depends on host and user namespace | Live macOS ARM64 guest reports 2 vCPU and 981 MiB visible memory for a 1 GiB configuration; PID limit remains unsupported |
| Workspace persistence and ownership | E2E persistence and daemon recovery passed; host ownership and full daemon matrix remain open | Podman 5.8.7 userns/recovery cases passed with documented capability skips; ownership matrix on target Linux remains open | Driver live test covers persistent ext4 overlay. Gateway now selects a guest-overlay workspace and omits the host path; Gateway-to-VM persistence and ownership acceptance remain open. Host workspace mounts are outside the OpenShell v0.1.4 VM model |
| Network isolation and policy broker | Docker 28 DinD E2E creates an IPv6 isolated network, verifies the sandbox IPv6 address, then confirms a reachable host listener at the conventional IPv6 gateway address cannot be reached from the sandbox. On one macOS ARM64 Docker Desktop host, a dual-stack control sandbox reaches host listeners over `host.docker.internal` (IPv4) and the host ULA IPv6 address; the proxy-isolated sandbox reaches neither. The driver also rechecks workspace directory identity immediately before Engine create, and a regression confirms replacement during preparation fails before workload creation. Atomic daemon-side path pinning and the wider host matrix remain open because the Engine API accepts a path rather than a directory handle. | Podman 5.8.7 lacks required blackhole-route/host-gateway isolation; Podman 6 target gate remains open | macOS ARM64 live probes deny host IPv4/IPv6 loopback and unmapped host-vsock port 5504; raw ICMP socket creation is denied with EPERM. Revision 2 reload/ack changes the broker decision; killing the broker stops the VM and allows durable recovery. Gateway acknowledgement integration, DNS and Linux KVM acceptance remain open |
| Interactive process and transfer | Package tests exist; live TTY, signal and large-file cases open | Package tests exist; live PTY, log driver and reconnect cases open | macOS ARM64 live driver verifies PTY stdin/output, 80×24→101×41 resize, Ctrl-C signal delivery and exit status; Gateway-to-VM and Linux KVM remain open |
| GPU / CDI / PID limits | Mapped where daemon supports the requested feature; target hardware evidence required | Native CDI/PID support varies by host; explicit capability rejection required otherwise | Explicitly unsupported pending implementation and target-host evidence |
| Recovery and cleanup | Gateway/daemon recovery suite exists; rerun on release runner | Rootless/rootful recovery suite exists; service and API restart lanes required | macOS ARM64 driver live test covers fresh-driver adoption, broker-loss stop/restart, worker-group crash/relaunch from persisted rootfs/overlay, stop/start and cleanup; Gateway restart recovery and orphan reaper remain open |
| Release artifacts | Build and published-binary smoke required | Build and published-binary smoke required | Linux KVM and macOS ARM64 package builds; live VM acceptance remains mandatory |

The MicroVM `ValidateGatewayRuntime` failure remains in place. On macOS ARM64,
driver-level `TestLiveMicroVMLifecycle` passed with libkrun/HVF, a Linux guest,
default-deny broker, exec/copy, supervisor readiness, an observed `on-failure`
restart, SSH relay with a TCP echo round-trip through guest loopback, a fresh
driver instance, stop/start and cleanup. This does not exercise Gateway
orchestration.
Builds or unit tests without a live hypervisor are not acceptance. Production
requires Gateway-to-workload acceptance on Linux KVM x86_64/aarch64 and full
Gateway acceptance on macOS ARM64.

## Current local run: 2026-10-16

The latest `task check` passes after `tools/modules.lock` and workflow refs
were aligned with published core v0.3.0 and proto v0.2.0. It covers workspace
and image pins, formatting, lint, vet/govulncheck, and all Go tests.
`task compat:inventory` (13/18 open or blocked), `task docs:strict`, and
`go test -race ./internal/mounts ./internal/docker` also pass. `GOWORK=off
task release:standalone` and `task release:artifacts` pass.
`task e2e:docker-desktop` passed its macOS ARM64 workspace and dual-stack
host-boundary lanes. `task e2e:docker` passed using isolated Docker-in-Docker,
including Gateway-to-supervisor relay lifecycle and
create/start/inspect/exec/stop/delete plus proxy allow/deny. `task e2e:podman`
passed against disposable Podman 5.8.7 for supported rootful/rootless
user-namespace and recovery cases; rootful keep-id skipped because nested crun
lacks mqueue, rootful no-map is rootless-only, rootless private maps are
unsupported, and Gateway/proxy lanes skipped because Podman 5 lacks blackhole
routes. Podman 6.1.3 in a nested Docker Desktop runner failed in the required
rootful user-namespace lane because nested cgroup controls were unavailable and
the `auto` mapping lacked a subordinate ID range, before reaching proxy tests.
Podman 6.1.3 lifecycle, service recovery, pull policy, digest pinning, and
rootless service recovery passed in the same nested environment; those checks
do not replace the missing complete user-namespace and target-host acceptance.
The merged driver v0.3.0 CI also passed rootful Podman 6.1.3 lifecycle,
service recovery, pull-policy and digest-pinning tests, plus rootless lifecycle,
service recovery and the `auto`/`host`/`keep-id`/`no-map` user-namespace matrix
on Ubuntu runners. Those lanes do not run proxy-backed host-gateway or Gateway
relay probes, so Podman 6 isolation acceptance remains open.
Driver PR #19, merged at `34060264b5627a0aac4b83d3aa4bb76f1eee9119`,
subsequently passed Podman 6.1.4 rootful and rootless proxy-isolation CI in
disposable service containers. Direct host-gateway denial, proxy allow/deny,
TCP/UDP probes, and broker-loss denial passed. These lanes do not include a
Gateway relay or native host; nested cgroup controls also exclude ID-map start
and resource-limit acceptance. Linux host validation is deferred by request.
A fresh Podman 6.1.4 AppleHVF machine completed guest boot but host SSH/API
forwarding through `gvproxy` was unreachable; the pre-existing machine still
fails Ignition while adding `core`. These attempts are not Podman 6 target-host
acceptance; a dedicated Linux runner is required. Driver v0.3.0 has been
published from merged PR #16; its CI passed Linux/macOS/Windows build and test,
lint/format, Linux VM artifact, and rootful/rootless Podman 6.1.3 lanes. Gateway
now resolves driver v0.3.0 and proto v0.2.0 from published tags, and its
standalone `GOWORK=off` test passes. The full `task release:published` still
fails only on CLI: it pins runtime v0.2.1 and SDK v0.2.0, which lack current
heartbeat/log and provider/service APIs. Core v0.3.0, proto v0.2.0, providers
v0.1.8, proxy v0.3.0, runtime v0.3.0, and driver v0.3.0 are published; SDK
v0.3.0 has a merged release PR awaiting its tag after the Gateway release.
Driver-level MicroVM lifecycle acceptance passed on macOS ARM64
using libkrun/HVF, covering exec/copy, deny policy, supervisor, SSH, driver
recovery and stop/start. The acceptance also kills the complete worker process
group, waits for durable state to report it stopped, restores the workload spec
through a fresh driver, and starts the VM again from its persisted rootfs and
overlay. It verifies `on-failure` by observing the supervisor begin a second
process instance after exit code 7. The Linux worker
namespace test binary cross-builds for amd64 and arm64, but the local Docker
runner blocks user namespace creation with `EPERM`; this is not Linux host
acceptance. Gateway-level MicroVM and Linux KVM acceptance remain open, so
Gateway runtime validation stays fail-closed.

The same macOS ARM64/HVF lane imports an Alpine rootfs through a Basic Auth
protected local OCI registry using an isolated Docker config. A test Docker
credential helper supplies auth to the VM's default keychain. The test starts a guest workload with
image `Entrypoint`/`Cmd`, `Env`, `WorkingDir` and `User`, and observes `/opt/app`,
UID 1000 and the image environment inside the running guest. Durable state omits
the test credential and image environment. After the registry tag moves, the
test stops and restores the workload and verifies that the original manifest
digest still supplies its config. Additional registry credential-helper
implementations and TLS/auth variants on target hosts, Linux KVM OCI pulls and
Gateway crash recovery remain open.

The latest macOS ARM64 live MicroVM run also exercises protocol v1, supervisor
readiness before workload start is acknowledged, durable retry-safe vsock
control-token rotation, and recovery after worker-group crash. Guest probes
deny host IPv4/IPv6 loopback and connection to unmapped host vsock port 5504;
the raw ICMP socket probe is rejected with EPERM on macOS ARM64. The guest
sees 2 vCPU and 981 MiB for a 1 GiB memory configuration. The live broker
reloads policy revision 2 and acknowledges it through the test harness: the
allowlisted endpoint passes policy evaluation, while another endpoint remains
denied. This does not exercise the Gateway status-report RPC. Workload identity now
travels through the post-hardening readiness handshake, is persisted by the VM
driver and is mapped to the OpenShell `WorkloadIdentity` status condition; unit
tests cover the Gateway projection, while Gateway-to-VM live verification is
still unavailable because VM creation remains fail-closed. Killing the live
policy broker stops the worker group, records durable `stopped` state and
permits relaunch after restoring the workload spec. Workspace host mounts, a
dedicated host UID, Gateway recovery and Linux KVM acceptance remain open. This driver-level evidence does not enable
the Gateway MicroVM runtime.
An additional subprocess test closes the parent-liveness pipe and verifies that
the worker terminates its process group; Gateway-launched VM and image-prep
workers receive this pipe so a crashed Gateway does not leave orphan VMs.

The same live lane verifies a full-duplex guest PTY: it observes the initial
80×24 size, resizes to 101×41, sends stdin, and interrupts a trapped command
with Ctrl-C, observing exit code 42. One-shot TTY exec also sees the OCI image
working directory and image environment. Gateway has unit coverage for the
interactive compute relay, but no Gateway-to-VM live run is counted while the
VM create gate remains closed.

## Release test order

The 2026-10-18 test-plan update moves the shared E2E lane to Podman 6.1.4 and
enables the userns matrix in rootful CI. Strict mode allows only rootful
`no-map` and rootless `private` as documented skips. The updated workflows have
not run yet; the historical Podman 5.8.7 result and earlier nested Podman 6
failures remain separate evidence and do not validate this new configuration.

1. `task workspace:check`, `task check`, and race/vet/vulnerability gates.
2. `task compat:pin-check`, `task compat:proto`, and `task compat:inventory`.
3. `task e2e:docker` on the supported Docker host matrix.
4. `task e2e:podman` on dedicated rootful and rootless Podman 6 runners; proxy,
   Gateway, and required userns cases must pass except for the two stated
   mode-specific skips.
5. MicroVM Gateway-to-workload and security tests on Linux KVM x86_64/aarch64
   and macOS ARM64; no synthetic pass when the hypervisor is unavailable.
   `task vm:live-darwin` runs driver-level libkrun/HVF acceptance on macOS
   ARM64; `quarshell-driver/scripts/run-live-kvm-acceptance.sh` runs the Linux lane.
6. Standalone Go/Python consumers, published artifact smoke, strict docs build,
   SBOM and signature checks.

The current compatibility register is
[`openshell_compatibility_gaps.json`](https://github.com/KLORQ/sandbox.dev/blob/main/docs/ru/reference/remaining-migration-work.md).
It records known gaps but cannot prove that every source behavior has been
entered. Open rows remain blocking until test evidence is attached.
