# VM Hypervisor.framework local acceptance

## Rootfs archive staging flow

- Reverified: 2026-10-22
- Focused checks: CLI staging into VM driver config, Gateway staging token
  consumption, SDK upload bounds, VM descriptor-relative path and archive
  validation; all PASS.
- `task vm:parity-check`: PASS (30 pinned entries).

The complete local rootfs tar flow is covered across modules: the CLI resolves a
local archive, the Gateway creates a workspace-bound single-use staging slot,
the SDK writes the bounded payload, CreateSandbox consumes the token and
substitutes the VM staging path, and the VM opens a regular file under its
request-scoped staging directory while enforcing archive size and expansion
limits. Path resolution rejects escapes, symlinked request directories and
archive files, non-regular files, oversized inputs, and replacement between
validation and use. Inventory: 20 implemented, 8 partial, 2 unsupported for
physical GPU/VFIO.

## Durable exec request admission

- Reverified: 2026-10-22
- Checks: Gateway sandbox admission and RPC behavior under `-race`: PASS;
  full workspace `task test`: PASS.

Both unary and interactive exec validate nonzero UUID request IDs and write an
admission fence atomically to the shared lifecycle store before starting a
runtime process. Concurrent duplicates cannot both launch. A duplicate after
SQLite close/reopen remains rejected; an unknown outcome stays fenced, while a
known terminal result starts a 24-hour retention period. After the retention
period, an expired completed record is deleted and replaced in one transaction.
The broader `vm.exec-pty-signals` row remains partial because interactive
non-TTY and full upstream PTY/signal acceptance are still outstanding.

## Gateway restart intent persistence

- Reverified: 2026-10-22
- Command: `go test ./internal/application/reconcile -run
  '^TestRunPreservesRunningAndStoppedIntentAfterSQLiteRestart$' -count=1`
  in `quarshell-gateway`
- Result: PASS

The test closes and reopens the SQLite lifecycle store and recreates the
Gateway application services. Reconciliation preserves a persisted running
sandbox without starting it again and stops a runtime that conflicts with a
persisted stopped intent. Inventory: 19 implemented, 9 partial, and 2
unsupported for physical GPU/VFIO.

## Gateway exec timeout propagation

- Reverified: 2026-10-22
- Command: `go test ./internal/transport/applicationgrpc -run
  'TestExecSandbox(ExecutionTimeoutCancelsRuntime|RPC)$' -count=1` in
  `quarshell-gateway`
- Result: PASS

The test runs a runtime exec that waits on its context and confirms a positive
Gateway `execution_timeout` cancels the runtime and returns gRPC
`DeadlineExceeded`. The live HVF test separately verifies that the guest
process is terminated after its exec context expires. Inventory: 18
implemented, 10 partial, and 2 unsupported for physical GPU/VFIO.

## Docker local OCI image export acceptance

- Reverified: 2026-10-22 21:37 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (22.93s) and
  `TestLiveLocalOCIEngineImport` PASS (1.18s)

Docker Engine 29 exports multi-platform images as OCI layouts. The VM importer
now requests only the target guest platform by immutable Engine image ID,
reads legacy Docker archives and OCI layout archives, selects the matching
platform manifest, and rejects unsafe archive paths and entry types. The live
test imported `docker.io/library/alpine:3.22` for `linux/arm64`, prepared the
VM rootfs image, and recovered its config by the same immutable image ID.
Inventory: 19 implemented, 9 partial, and 2 unsupported for physical GPU/VFIO.

## Workload identity parity and full workspace retest

- Reverified: 2026-10-22 21:16 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (22.82s); required Go test gate PASS
- Workspace gate: `task test` PASS across all Go workspace modules
- Parity/format gates: `task vm:parity-check` (30 entries), `task fmt:check`,
  and `git diff --check` PASS

VM start now validates policy `run_as_user`/`run_as_group` selectors against
the guest's resolved non-root identity before releasing the workload start
gate. Gateway persists the observed runtime UID/GID and exposes a
`WorkloadIdentity` condition. Focused VM and Gateway tests cover matching and
conflicting policy selectors, durable identity persistence, and the gRPC
condition. Live HVF acceptance verified identity `1000:1000` and that the
persistent `/sandbox` owner remains `1000:1000` across stop/start. Inventory:
19 implemented, 9 partial, and 2 unsupported for physical GPU/VFIO. Lifecycle
extension parity is implemented across registration, activation labels,
capability validation, launch configuration/hooks, restore hooks, failure
cleanup and deletion; unit tests cover ordering, invalid requests, and cleanup,
and the live HVF acceptance activates a guest-init extension.

## Latest retest: Gateway runtime config and proxy path

- Reverified: 2026-10-22 20:47 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (23.96s); required Go test gate PASS
- Related checks: full workspace `task test`, VM parity inventory (30 entries),
  `task fmt:check`, and `git diff --check` PASS

The live HVF lifecycle again verified boot/readiness, identity and ownership,
default-deny egress, authenticated HTTPS CONNECT allow/revoke, worker recovery,
stop/start, broker-loss shutdown, pinned OCI pulls, natural workload exit, and
Gateway credential/process-event flow. Gateway TOML selection and driver table
registration now feed corporate proxy settings into the VM broker; focused
tests verify config parsing, operator precedence, and fail-closed behavior when
the policy broker is missing. The configuration handoff itself is unit-tested;
the live proxy run still supplies its proxy fixture explicitly.

The VM inventory was 14 implemented, 14 partial, and 2 unsupported for
physical GPU/VFIO hardware. This remains partial OpenShell VM parity.

## SPIFFE Workload API config handoff

The VM TOML driver config now accepts the pinned TCP endpoint only with
`provider_spiffe_allow_guest_tcp = true`, validates a concrete address and
nonzero port, and injects the endpoint into the policy broker using
`OPENSHELL_PROVIDER_SPIFFE_WORKLOAD_API_SOCKET`. Sandbox-supplied values for
this operator-owned setting are removed. VM config and precedence tests pass;
the token-grant resolver's endpoint-bound dynamic credential test covers the
proxy's consumer path. A second `task vm:live-darwin` run after this change
passed (23.98s) with live HVF, proxy allow/revoke, and Gateway credential/event
flow. That live run does not configure a real SPIFFE Workload API endpoint;
the endpoint handoff is unit-tested. Full workspace tests, 30-row parity,
format and diff checks also passed after the change. Inventory at that retest: 14 implemented,
14 partial, 2 GPU/VFIO unsupported. The parity review also promoted lifecycle
create/start/stop/delete, overlay persistence, Gateway create/readiness,
bootstrap image preparation, and policy revision acknowledgement based on the
existing live HVF lifecycle and recorded full Gateway composition smoke.

- Verified: 2026-10-22 (Europe/Samara)
- VM source snapshot: `7d26991760bc9932067dc77db9e00005c934b30a`
- Host: local Apple Silicon Mac, `darwin/arm64`, `kern.hv_support=1`
- Runtime: libkrun 1.19.6, libkrunfw 5.6.2, e2fsprogs 1.47.4, Docker 29.8.0
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (10.87s); required Go test gate PASS

The live lifecycle built the worker and Linux/arm64 guest artifacts, created a
pinned Alpine rootfs, and exercised boot/readiness, process identity and
ownership, policy egress and revision acknowledgement, worker recovery,
stop/start, broker-loss shutdown, authenticated registry pull, natural workload
exit, and cleanup. The run also verified that credentials and image environment
are absent from durable state. The production start path atomically published a
mode-0600 isolation descriptor and re-read it against the current sandbox,
session, generation, rootfs/runtime digests and private mTLS identities; the
live acceptance asserted the descriptor from durable state, verified that
worker adoption preserves the current runtime generation, and verified that an
explicit Stop/Start rotates it instead of reusing the previous descriptor
identity.

This is host-level HVF evidence for the VM driver. Gateway VM creation remains
fail-closed pending host supervisor/isolation integration (G04); this run does
not verify KVM, Apple signing/notarization, or GPU/VFIO.


## Retest after module extraction

- Reverified: 2026-10-22 10:54 Europe/Samara
- VM package: `quarshell-driver-vm/vm`
- Worker and guest-agent commands: `quarshell-driver-vm/cmd/`
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (12.63s); required Go test gate PASS

The host worker, guest agent, lifecycle implementation, parity inventory, and
their tests now live in the dedicated `quarshell-driver-vm` module. The live
HVF lifecycle was rebuilt and rerun from that module after the move.

## Contract retest

- Reverified: 2026-10-22 11:54 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (13.75s); required Go test gate PASS
- Additional contract check: `task vm:parity-check` PASS (30 unique entries: 29 upstream parity features and one supplemental host-supervisor blocker)

The live test again exercised VM boot/readiness, process identity, policy
default-deny and revision acknowledgement, worker adoption, stop/start,
broker-loss shutdown, authenticated registry access, and natural workload exit.
The descriptor was published and checked against durable boot identity. The
guest workload start gate now travels over an inherited one-shot pipe through
the VM agent, guest supervisor, and `cautem-init`; the live test passed after
the init process dropped to its non-root workload identity. Per-boot control
identity now uses the canonical UUID session form expected by OpenShell. This
is evidence for the VM driver's host lifecycle and its local control paths;
the auth-bundle unit tests separately verify the paired credentials and epoch
bindings.

The contract review found that this does not establish Gateway-to-VM parity.
OpenShell v0.1.4's `spawn_host_supervisor` starts a host role with a backend
descriptor file, supervisor auth bundle, admitted backend, Gateway endpoint,
and resolved sandbox identity. The private auth bundle now matches the upstream
JSON field schema and carries the paired tokens, expiry, Gateway runtime
generation, Gateway-owned session UUID/rotation, and auth epoch; the proxy
validates the bundle session against the same durable launch identity in the
local descriptor. The Go token signer and claim profile now match the upstream
JWT contract, including duplicate-field rejection. The relay routes the Gateway
bearer to Gateway API and the Sandbox bearer to supervisor relay methods; it
does not launch the upstream host role or attach the VM descriptor through the
backend registry.
`quarshell-core/isolation` is a separate reduced lifecycle contract and lacks
the upstream backend's TCP/DNS mediation and running-boundary process/exec/
loopback interfaces. Gateway VM runtime remains intentionally fail-closed, so
this HVF pass is not G04 evidence.

## Credential/session binding retest

- Reverified: 2026-10-22 11:59 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (13.98s); required Go test gate PASS
- Contract tests: descriptor-bound paired auth bundle, Gateway session UUID and
  runtime generation binding, token epoch checks, and affected Gateway
  application/transport packages PASS

The credential launch path now carries the same Gateway-owned session UUID and
`g`-prefixed runtime generation through the VM mTLS identity, isolation
descriptor, and upstream-shaped host auth bundle. The live test covers the
current uncredentialed HVF lifecycle; the paired-credential path is covered by
contract/unit tests, not by a live Gateway-to-VM launch. The Go token issuer now
uses EdDSA, upstream JOSE types and the exact strict session claim set, with
public verification-key export. The Gateway ID and verification key set now
travel with launch credentials; VM validates Ed25519 key IDs and installs the
set into the guest root-owned control directory over authenticated vsock before
starting the workload. The guest supervisor now loads the file and validates
its permissions, strict key records, unique IDs, Gateway identity, and Ed25519
PEM. It does not yet use these keys to verify JWTs inside the full boundary
protocol. Targeted Go tests cover these checks and removal of the private config
path from workload env; they do not exercise a live Gateway-to-VM launch. Host
backend registry attach/confirm and the full upstream isolation backend contract
remain unimplemented; G04 is still open.

Pinned-source contract review: `openshell-core/src/config.rs` defaults
`GatewayJwtConfig.gateway_id` to `openshell`; JWT `iss` and Gateway `aud` use
that stable ID rather than the display name. The Gateway now exposes
`--gateway-id` / `QUARSHELL_GATEWAY_ID` (default `openshell`), and the local
default issuer follows it. The verifier also accepts upstream's supported
non-expiring `exp=0` form. `go test -count=1 ./...` in `quarshell-gateway` and
`task vm:parity-check` PASS.

Paired bearer routing fix: Go host proxy now extracts both fields from the
descriptor-bound bundle. Gateway API/policy calls use `gateway_token`; the
supervisor relay uses `sandbox_token` for `ConnectSupervisor` and `RelayStream`.
`go test ./internal/service -run 'SandboxToken|ProxyToken' -count=1` in
`quarshell-cli` PASS. Initial session rotation 1 matches pinned OpenShell
v0.1.4 (`sandbox_jwt.rs` default and VM propagation into `VmBoundarySpec`);
successor rotation/replay and host backend attach/confirm remain open.

The Go relay refresh path now selects `RefreshSandboxTokenResponse.SandboxToken`
for its next Sandbox Protocol connection (and falls back to `Token` for older
single-bearer Gateways). `go test ./relayclient -run
'SessionRefreshes|Isolation' -count=1` in `quarshell-runtime` PASS.

The same refresh now publishes the Gateway bearer to the long-running host
proxy, its HTTP Gateway client, and per-RPC Gateway credentials. `go test
./relayclient -count=1`, `go test ./go/klorq ./internal/gatewayclient -count=1`,
`go test ./proxy -count=1`, and `go test ./internal/service -count=1` PASS in
their Go modules. This refresh path still lacks durable session rotation and
replay lineage.

Race-detector retest PASS for all four affected packages: runtime relayclient,
SDK `go/klorq` plus `internal/gatewayclient`, proxy, and CLI `internal/service`.

JWT strict decoding also now rejects duplicate object keys, matching the
upstream serde claim model instead of accepting Go's last-value-wins behavior.
`go test ./internal/application/token -count=1` in `quarshell-gateway` PASS.

## Follow-up: strict non-expiring expiry claim

- Reverified: 2026-10-22 14:29 Europe/Samara
- Source revision: `26feb5a127c71c97dc656861ad4a710533737f8f`
- Commands: `go test ./internal/app/supervisor -count=1`; `task vm:parity-check`; `task vm:live-darwin`; supervisor test cross-compiles for Linux/ARM64 and Windows/AMD64
- Result: all local commands PASS; live `TestLiveMicroVMLifecycle` PASS (12.59s)

The guest verifier requires the `exp` claim to be a JSON integer. OpenShell's
`exp: 0` remains accepted as its non-expiring profile; `exp: null` is rejected
instead of being mistaken for zero by Go's JSON decoder. The live HVF test
continues to cover the direct VM lifecycle only. No live Gateway paired-token
launch was performed, so this does not close G04 or the outstanding VM parity
rows. GitHub secret-scan run `39763539106` failed before any job steps; its
details are recorded in `plans/REMAINING.md`.

After pushing this snapshot, a manual hosted platform matrix was dispatched:
run `39763802132`. Its Linux KVM, macOS HVF, and Windows MXC jobs all ended
before the first step and produced no logs; this gives no hosted platform
validation. Secret scan `39763756651` behaved the same way. Local one-root
history Gitleaks remains the available secret scan evidence.

## JWT claim decoding hardening

- Reverified: 2026-10-22 14:35 Europe/Samara
- Commands: `go test ./internal/app/supervisor -count=1`; supervisor test cross-compiles for Linux/ARM64 and Windows/AMD64; `task vm:parity-check`; `scripts/check-docs-strict.sh`
- Result: all commands PASS

The guest verifier now requires both `iat` and `exp` to be JSON integers. Tests
reject missing/null `iat` and null `exp`; the upstream non-expiring `exp: 0`
profile remains accepted. This is unit/contract evidence and does not exercise
Gateway paired credentials in a live HVF VM.

The full Go workspace retest after these verifier changes also passed:
`task test`, `task vm:parity-check`, `task vm:artifact-check
vm:kvm-script-check`, Linux/ARM64 and Windows/AMD64 supervisor test
cross-compiles, and `git diff --check`. GitHub secret scan
`39764231486` ended before its first step and had no job logs; local Gitleaks
on the one-root history passed.

## Latest local HVF retest

- Reverified: 2026-10-22 12:15 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (15.38s); required Go test gate PASS

This retest confirms the VM driver's boot, policy, recovery, workload gate,
registry pull, and shutdown paths after the Gateway token and relay changes.
It still starts through the direct VM driver path. A live launch using real
Gateway-signed paired credentials has not been exercised, so G04 remains open.

## Current-checkout HVF retest

- Reverified: 2026-10-22 13:31 Europe/Samara
- Command: `task vm:live-darwin`
- Result: `TestLiveMicroVMLifecycle` PASS (14.15s); required Go test gate PASS

The current checkout passed the full local HVF lifecycle after the paired
credential contract was extended with Gateway identity and validated Ed25519
public verification keys. The VM driver now installs that key set into a
root-owned mode-0600 guest supervisor-control file through the authenticated
guest control channel. The direct HVF acceptance does not launch with Gateway
paired credentials and does not establish end-to-end Gateway-to-VM parity.
The retest also caught and fixed a cross-package reference to a Linux-only
private constant by moving the guest credential directory into the shared
VM guest protocol contract.

## macOS ARM64/HVF retest

- Reverified: 2026-10-22 13:40 Europe/Samara
- Source revision: `900b69e2c516238b584d83a02f7f4814b98e86f3`
- Host: Apple Silicon, macOS Darwin 27.0.0 (ARM64), HVF/libkrun
- Command: `task vm:live-darwin`
- Result: PASS; `TestLiveMicroVMLifecycle` and required Go test gate passed

This run rebuilt the macOS worker and Linux/ARM64 guest agent. Live checks
covered guest boot/readiness, non-root workload identity, CPU/memory reporting,
proxy CA installation, default-deny HTTP and raw-socket egress, unmapped vsock
denial, policy revision delivery, worker recovery, persistent workload image
identity, natural process exit, explicit Stop/Start, private registry auth, and
worker shutdown after broker loss. It did not launch through Gateway with paired
session credentials and does not close G04.

## macOS retest with guest verification-key consumer

- Reverified: 2026-10-22 13:46 Europe/Samara
- Command: `task vm:live-darwin`
- Result: PASS; `TestLiveMicroVMLifecycle` and required Go test gate passed
- Additional macOS package gates: `go test ./vm ./vmguest -count=1` in
  `quarshell-driver-vm` and `go test ./internal/app/supervisor -count=1` in
  `quarshell-runtime` PASS

This retest rebuilt the current runtime assets after adding guest-side
verification-key file validation. HVF lifecycle probes passed again. The live
lane still does not perform a Gateway-issued credential launch, so guest key
delivery remains covered by Go contract tests rather than live VM evidence.
The key-file consumer also checks directory/file ownership against the
supervisor effective UID/GID; `go test ./internal/app/supervisor -count=1` was
rerun on Darwin after this check and passed. The full VM package suite also
passed on Darwin after the live acceptance.

## macOS ARM64/HVF recheck

- Reverified: 2026-10-22 13:49 Europe/Samara
- Command: `task vm:live-darwin`
- Result: PASS; `TestLiveMicroVMLifecycle` passed in 12.24s and the required Go
  test gate passed
- Additional gates: full `quarshell-runtime` tests PASS; Linux ARM64 and
  Windows supervisor package cross-compiles PASS; `task vm:parity-check
  docs:strict` PASS

This run rechecked the existing driver-level lifecycle after the guest key
bundle validator and its Unix ownership checks were added. It covered HVF boot,
egress denial, policy revision acknowledgement, recovery, registry pull,
Stop/Start, broker-loss shutdown, and workload exit. It still did not launch
the VM through Gateway with paired session credentials; key-bundle validation
is covered by supervisor package tests, and G04 remains open.

## Guest Sandbox JWT verification and latest HVF rerun

- Reverified: 2026-10-22 14:00 Europe/Samara
- `go test ./...` in `quarshell-runtime`: PASS
- Linux ARM64 and Windows AMD64 supervisor test cross-compiles: PASS
- `go test ./vm` in `quarshell-driver-vm`: PASS
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` passed in 11.66s
- `task vm:parity-check`: PASS (30 pinned OpenShell v0.1.4 entries)
- `task docs:strict`: PASS

Guest startup now consumes the private Ed25519 key set and verifies the
Sandbox Protocol JWT against the active Gateway ID, sandbox ID, runtime
generation, and auth epoch before spawning workload. Unit coverage checks a
valid signed session and rejects a sandbox mismatch, epoch mismatch, bad
signature, duplicate claims, wrong JOSE type/algorithm, expired/future tokens,
and the wrong component. Bearer and internal session identity variables are
stripped from the child workload environment. The HVF run is still
driver-level and does not exercise the Gateway paired-credential launch or
close G04.

The pushed snapshot's GitHub secret-scan run
([39760492432](https://github.com/KLORQ/quarshell/actions/runs/39760492432))
and manually dispatched platform matrix
([39760554090](https://github.com/KLORQ/quarshell/actions/runs/39760554090))
both failed before executing steps. GitHub's jobs had no steps and stopped at
runner startup due to the account billing/spending-limit block; local tests and
Gitleaks are the available evidence for this snapshot.

The strengthened token verifier test set is on code snapshot `5f58ad7`.
Its push-triggered GitHub secret scan
([39762004056](https://github.com/KLORQ/quarshell/actions/runs/39762004056))
also stopped before any job steps due to the same account billing/spending
limit block. Local full-history Gitleaks scanned the single root commit
(18.20 MB) and found no leaks.

The full Go workspace gate `task test` also passed on this snapshot across all
module directories, including VM, driver, Gateway, runtime, CLI, core, and
proxy packages.

## Latest local Apple Silicon/HVF acceptance

- Reverified: 2026-10-22 14:21 Europe/Samara
- Command: `task vm:live-darwin`
- Result: PASS; `TestLiveMicroVMLifecycle` passed in 11.41s and the required
  Go test gate passed

Probes covered guest boot/readiness, resource and non-root identity,
default-deny HTTP/raw egress, unmapped vsock, policy revision acknowledgement,
worker adoption, stop/start, broker-loss shutdown, registry authentication and
digest pinning, natural workload exit, and cleanup. This remains driver-level
HVF evidence; it does not exercise Gateway-issued paired credentials or host
isolation-backend attach/confirm, so G04 remains open.

## Latest local Apple Silicon/HVF acceptance after JWT claim hardening

- Reverified: 2026-10-22 14:22 Europe/Samara
- `go test ./internal/app/supervisor -count=1` in `quarshell-runtime`: PASS
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` and required Go gate passed
- `task vm:parity-check`: PASS (30 pinned OpenShell v0.1.4 entries)
- `git diff --check`: PASS

The Sandbox JWT verifier rejects a missing `exp` claim, zero/missing `iat`,
expired or overlong lifetimes, and accepts the upstream `exp: 0`
non-expiring profile. Supervisor tests also confirm partial session
credential environment fails closed. This run completed at 14:21, before the
partial-environment check was added; the next section records the 14:24 rerun.
The live HVF lane remains driver-level and does not exercise Gateway
paired-credential launch or host isolation backend attach/confirm; G04 remains
open.

## Latest HVF acceptance after partial-session fail-closed check

- Reverified: 2026-10-22 14:24 Europe/Samara
- `go test ./internal/app/supervisor -count=1`: PASS
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` passed in 13.14s and
  the required Go gate passed
- `task vm:parity-check`: PASS (30 pinned OpenShell v0.1.4 entries)
- `git diff --check`: PASS
- Full `task test` across Go workspace modules: PASS
- Linux ARM64 and Windows AMD64 supervisor test cross-builds: PASS

Guest supervisor now rejects partially configured Sandbox Protocol session
credentials instead of skipping JWT verification when Gateway ID is absent.
The JWT implementation preserves pinned OpenShell semantics: `exp` is required,
`exp=0` is non-expiring, and expiry checks apply when `exp` is positive. This
remains a supervisor-unit-tested contract; the HVF run remains at the driver
lifecycle layer, so Gateway paired-credential/host-backend acceptance and G04
remain open.

## Final rerun after matching pinned JWT edge semantics

- Reverified: 2026-10-22 14:25 Europe/Samara
- `go test ./internal/app/supervisor -count=1`: PASS
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` and required Go gate passed
- `task vm:parity-check`: PASS (30 pinned OpenShell v0.1.4 entries)
- `git diff --check`: PASS

The Go verifier follows the pinned Rust contract: `exp` must be present, zero
means non-expiring, and a positive value is checked for expiry and maximum TTL.
Partial session credential injection fails before workload launch. This rerun
does not provide Gateway→VM host-supervisor attach/confirm evidence; G04 is
still open.

On current pushed code snapshot `66b1e6d`, local full-history Gitleaks scanned
one root commit (18.21 MB) with no leaks. GitHub's push-triggered secret scan
([39762975463](https://github.com/KLORQ/quarshell/actions/runs/39762975463))
and manually dispatched platform matrix
([39763010550](https://github.com/KLORQ/quarshell/actions/runs/39763010550))
stopped before their first step because of the account billing/spending limit
block.

## HVF exec timeout cancellation probe

- Reverified: 2026-10-22 14:37 Europe/Samara
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` passed in 17.30s and required Go gate passed
- New probe runs a guest `sleep; touch marker` under a 150 ms caller deadline, confirms Exec returns an error, then verifies after the original sleep interval that the process group did not create the marker
- `go test ./vm ./vmguest -count=1`: PASS; parity inventory and `git diff --check`: PASS
- This exercises the macOS HVF driver and Linux guest agent; it does not close the separate Gateway host-supervisor/G04 gap

After push, hosted platform matrix [`39764654224`](https://github.com/KLORQ/quarshell/actions/runs/39764654224) started Linux KVM, macOS HVF, and Windows MXC jobs, but all terminated before their first step with no logs. Local HVF remains the only live platform evidence for this snapshot.

## Host bootstrap contract audit

- Rechecked: 2026-10-22 14:42 Europe/Samara
- Pinned sources: `tools/upstream/openshell-v0.1.4/crates/openshell-driver-vm/src/isolation/mod.rs`; `.../driver.rs::spawn_host_supervisor`; `tools/upstream/openshell-v0.1.4/rfc/0012-isolation-backend/README.md`
- Local sources: `quarshell-driver-vm/vm/spec.go::validateGatewayRuntime`; `quarshell-driver-vm/vm/libkrun_ffi_unix.go`; `quarshell-driver-vm/vm/lifecycle.go::startLocked`; `quarshell-core/isolation/contract.go`

The local libkrun setup maps the guest to vsock endpoints, calls
`krun_add_vsock` with transparent socket interception disabled, and does not
configure a NIC. `startLocked` waits for the guest supervisor and workload
initializer to report readiness/non-root identity, then sends
`confirm-workload` to release the inherited start gate. These are useful native
facts for a backend attach/confirm implementation.

This audit was superseded by the M6c registry implementation below. The
production credential-aware VM start path now attaches the verified descriptor
to `vmIsolationBackend`, checks worker/broker health, guest control and mTLS
identity, validates non-root readiness evidence, and releases the guest gate
through `ConfirmedBoundary.Start`. The full Gateway composition and broader
upstream running-boundary operations remain outside this local HVF probe.

## Gateway credential and registry attach HVF probe

- Reverified: 2026-10-22 15:15 Europe/Samara
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` passed in 22.20s and the required Go test gate passed
- `CAUTEM_VM_TEST_GATEWAY_CREDENTIALS_ONLY=1 task vm:live-darwin`: PASS; paired credential/registry acceptance passed in 5.31s and required Go test gate passed
- Live mock Gateway checked Sandbox bearer on `ConnectSupervisor`, Gateway bearer on policy revision status, instance rotation, and fresh mTLS control identity. The VM driver attached its private descriptor through `isolation.Registry`, performed confirmation, released the one-shot guest start gate, and executed a guest command afterward.
- This uses a mock Gateway and local unsigned test identity path. It does not exercise the complete Gateway composition, trusted macOS Team ID signing, credential refresh/replay, KVM, or upstream process/exec/loopback operations on a running boundary. G04 remains open.

## Paired Gateway credential and registry retest

- Reverified: 2026-10-22 15:16 Europe/Samara
- Host: local Apple Silicon Mac, `darwin/arm64`, Hypervisor.framework
- Commands: `task vm:live-darwin`; credentials-only run with
  `CAUTEM_VM_TEST_GATEWAY_CREDENTIALS_ONLY=1 task vm:live-darwin`
- Results: full `TestLiveMicroVMLifecycle` PASS (23.24s test / 23.61s Go
  package); latest paired-credential `TestLiveMicroVMLifecycle` PASS
  (4.82s test / 5.23s Go package). Both runs passed after the broker PID
  accessor refactor.

The paired-credential scenario launches the actual `cautem proxy` binary and a
live HVF VM with EdDSA Gateway/Sandbox session tokens, the private descriptor
bound auth bundle, and the guest verification-key set. The mock OpenShell
Gateway verified that `ConnectSupervisor` carried the Sandbox bearer and
`ReportPolicyStatus` carried the Gateway bearer, observed a new supervisor
instance after workload start, and acknowledged policy revision 2. Guest exec
confirmed that the workload passed the one-shot start gate.

VM startup now passes the verified descriptor through the production
`quarshell-core/isolation.Registry`. The `cautem/vm` backend checks the live
worker and policy broker, broker health, guest control reachability, and the
per-generation mTLS identity; it commits those observations and the non-root
guest readiness identity into generation-bound evidence. `ConfirmedBoundary`
rechecks those controls and releases the guest start gate only after
confirmation. `validateGatewayRuntime` now returns after successful host
preflight instead of unconditionally rejecting Gateway VM launches.

For the separate policy-egress probe, the test helper intentionally configures
an unreachable corporate proxy at `127.0.0.1:1`; an HTTP 502 after a newly
allowed policy decision proves the rule passed evaluation but does not prove
successful upstream data transfer. The `vm.policy-egress` parity row remains
partial on this evidence.

This remains local HVF evidence against a mock Gateway. It does not exercise
the full Gateway composition, trusted macOS Team ID signing, credential
rotation/replay, KVM, or upstream process/exec/loopback operations on a running
isolation boundary. G04 remains open pending those gaps.

## Current-tree Mac retest and Gateway integration probe

- Reverified: 2026-10-22 (Europe/Samara)
- `task test`: PASS across all Go workspace modules after the Gateway config forwarding changes.
- `task vm:parity-check`: PASS (30 entries, pinned OpenShell v0.1.4 commit).
- `task vm:live-darwin`: PASS; real Apple Silicon/HVF `TestLiveMicroVMLifecycle` passed in 23.23s, including workload boot, policy enforcement/revocation, worker adoption, stop/start, broker-loss shutdown, and Sandbox/Gateway credential exchange through the live test harness.
- Fixed VM launch-spec cloning to retain `GatewayURL`, image and all other create/network fields. Fixed Gateway gRPC `CreateSandbox` to persist and forward `driver_config` and added tests for backend config selection and persistence.
- A temporary full Gateway + CLI probe now provisions the VM and launches the guest supervisor, but the supervisor rejects the Sandbox JWT because one or more session claims do not match its active session. The verifier now logs only boolean field-match diagnostics (never token or claim values) to identify the mismatch on the next full-composition run. This is still open integration work; the VM driver live acceptance alone does not prove Gateway VM create/start end-to-end.
- Trusted Developer ID signing/notarization and Linux KVM remain host credentials/capability gates; macOS Hypervisor.framework lifecycle is locally verified.

## Policy revocation and current-tree HVF retest

- Reverified: 2026-10-22 15:20 Europe/Samara
- `go test ./...` in `quarshell-driver-vm`: PASS
- `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...`: PASS
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`: PASS
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` passed in 22.43s
- The live guest received a policy revision allowing `policy-probe.invalid`,
  made a request that passed policy evaluation, then received revision 3 with
  no network policies. The same request returned HTTP 403 after the broker
  acknowledged revision 3. This directly exercises revocation of a previously
  granted destination.
- Backend-native evidence now records the no-NIC/vsock-only network boundary,
  the parent-liveness path that stops the worker after broker loss, and the
  guest-only disk confinement path before its digest is projected into the
  common isolation contract.

This closes the live policy-revision revocation check on macOS HVF. The broader
VM parity inventory remains incomplete; see `plans/REMAINING.md` for the open
and partial rows.

## Worker trust and current-tree workspace verification

- Reverified: 2026-10-22 15:41 Europe/Samara
- `task test`: PASS across every Go workspace module.
- `task vm:parity-check`: PASS (30 entries, pinned OpenShell v0.1.4 commit).
- `git diff --check`: PASS.
- VM worker trust accepts a locally built ad-hoc signed macOS worker only when
  its exact SHA-256 is configured; the binary is checked for regular-file
  identity, safe permissions, stable contents during hashing, and a valid
  ad-hoc signature. A focused trust unit test and Windows AMD64 package
  cross-compile passed earlier in this work session.
- A temporary full Gateway + CLI VM create probe reached the Gateway VM
  runtime but failed at `sandbox.create: provision runtime`; enabling the
  test Gateway's explicit `allow_driver_config` setting did not change the
  failure. The Gateway surfaced no underlying driver error, so this probe
  does not establish full-composition VM success. The temporary Gateway,
  router, sandbox state, CLI binary, and CLI config created for this probe
  were removed afterward.

The local worker hash pin is a development trust path and does not replace
Developer ID signing or notarization for distribution. Full Gateway
composition remains an open M6c requirement alongside the partial upstream
VM features listed in `plans/REMAINING.md`.

## VM log follow regression fix

- `Driver.Logs(follow=true)` now tails the guest console and worker log
  concurrently. Previously, EOF while following the console log kept the
  method waiting there and it never reached the worker log.
- `TestVMStartExecCopyLogsStopDeleteLifecycle` appends to the worker log after
  follow starts and verifies the live update is delivered.
- Verified with `go test ./vm -run
  '^TestVMStartExecCopyLogsStopDeleteLifecycle$' -count=1` on macOS ARM64.

The `vm.logs-watch` parity row remains partial because the upstream
`WatchSandboxes` lifecycle snapshot stream still has no Go VM implementation.

## macOS HVF retest after log-follow and Gateway sandbox-ID fixes

- Latest `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` passed in
  21.83s after also fixing single-source follow cancellation.
- `task test`, `task vm:parity-check`, `task fmt:check`: PASS.
- Linux ARM64 and Windows AMD64 `go build ./...`: PASS.
- The production proxy environment now replaces an untrusted sandbox ID with
  the Gateway-issued sandbox ID used by paired runtime credentials. Regression
  tests cover duplicate and empty caller values.
- The VM lifecycle test verifies live follow of a late worker-log append and
  worker-log access when the console log is absent; it passes under `-race`.

## Full Gateway composition smoke on Apple Silicon/HVF

- Initial probe: 2026-10-22 16:47 Europe/Samara; owner-release/log-delivery retest: 16:56 Europe/Samara
- Host: Apple Silicon, macOS ARM64, HVF/libkrun
- Commands: `task test`; `task vm:parity-check`; `task vm:live-darwin`; real Gateway HTTP create/start/stop/restart requests
- Result: workspace tests and parity inventory PASS; live HVF acceptance PASS (23.02s); Gateway create/start/stop/restart returned HTTP 200, including the repeated Start.

The Gateway composition smoke exercised the real Gateway, VM runtime adapter,
macOS worker, guest bootstrap and host proxy. Start reached the guest
supervisor and opened the workload lifecycle. Stop returned `stopped`, then a
new Start advanced runtime generation and credential epoch to 3 and returned
`running`. This exposed that the Gateway's 45-second supervisor owner lease
was retained after an intentional Stop. The runtime now releases the owner
lease after a successful explicit Stop; unexpected stream loss retains it.
The repeated live Start returned HTTP 200 and the new supervisor instance
connected immediately without lease-conflict retries.

This supersedes the earlier probe entries above that ended at
`sandbox.create: provision runtime`: launch-spec cloning and Gateway
`DriverConfig` forwarding were fixed, paired bearer routing was corrected, and
the full Gateway create/start/stop/restart path now succeeds.

Supervisor process events use the Sandbox bearer for `PushSandboxLogs`, as in
the pinned OpenShell auth contract. The current Gateway transport accepts
sandbox-scoped batches, caps requests at 1000 lines, retains the last 2000
lines per sandbox in memory, and filters `GetSandboxLogs` by count, source,
timestamp and level. Gateway tests check sandbox binding; the live paired
credential HVF fixture observed the process event. `WatchSandbox` and durable
log retention remain incomplete. The 17:02 composition probe below predates
the SDK gRPC endpoint wiring and reported HTTP-vs-gRPC warnings; the latest live
acceptance resolves secrets over the separate gRPC listener. The VM
parity inventory remains 2 implemented, 24 partial, 2 unsupported, and 2 open.

## Host gRPC endpoint contract and final local gates

The pinned VM driver requires the Gateway gRPC endpoint as host deployment
configuration and reads `OPENSHELL_GRPC_ENDPOINT`. The Go backend previously
validated the `grpc_endpoint` runtime setting but discarded it, allowing the
guest proxy to fall back to the public HTTP URL. `HostConfig` now retains the
setting, validates that it is an HTTP(S) authority without an API path, honors
the upstream environment variable, and prefers the host endpoint when building
the guest proxy environment. Sandbox-level endpoint configuration remains a
fallback for this product's existing per-sandbox integration.

On 2026-10-22, `task test`, `task vm:parity-check` (30 entries),
`task fmt:check`, `git diff --check`, and `task vm:live-darwin` all passed;
the latest live HVF lifecycle took 21.93s. The initial isolated `go test ./vm` had a
single timing failure in `TestParentLivenessStopReturnsWhileParentAlive`; a
three-count rerun of that test together with the new endpoint tests passed,
and the full workspace test gate passed afterward. Cross-builds for Linux
ARM64 and Windows AMD64 were also run for the VM, Gateway, and runtime modules.

## Latest real Gateway VM, log, and policy-auth retest

- Reverified: 2026-10-22 17:11 Europe/Samara
- Host: Apple Silicon, macOS ARM64, HVF/libkrun
- Result: real Gateway create/start/stop/start all returned HTTP 200; runtime generations advanced 2 → 3 and both starts reported `running`. gRPC `GetSandboxLogs` returned the VM supervisor's launch/exit/launch process events (three sandbox-filtered lines; buffer total was 5).

The latest Gateway and CLI proxy binaries were rebuilt from the working tree.
The host proxy connected both supervisor instances to the Gateway gRPC listener.
The Gateway returned process events bound to the exact Gateway sandbox ID;
`PushSandboxLogs` emitted no authentication or unimplemented errors. Explicit
Stop released the previous owner, so the immediate restart connected without
lease-conflict retries. Optional secret resolution now reaches the correct
gRPC endpoint and returns `Unimplemented` for the still-missing
`GetSandboxProviderEnvironment` handler rather than a transport/auth error.

The Gateway's `ReportPolicyStatus` handler now accepts only a Sandbox bearer
whose sandbox ID, running runtime generation, and credential epoch match the
durable sandbox record. It records loaded/failed status for the active policy
revision; targeted Gateway tests cover both outcomes and stale/cross-sandbox
rejection. The live real Gateway lifecycle above had no active Gateway policy
revision configured, so that real-process policy result is covered by unit
integration tests plus the separate paired-credential live fixture, not this
manual smoke.

The Go SDK now accepts a dedicated gRPC endpoint, and the CLI proxy configures
it from `CAUTEM_GATEWAY_GRPC_ENDPOINT`; the live fixture confirms the proxy
reaches the correct Gateway gRPC listener. `GetSandboxProviderEnvironment`
remains unimplemented: credential values must stay withheld until sandbox
provider attachment, endpoint authorization, credential identity/handles and
supervisor installation are connected end to end. Gateway unary auth admits
Sandbox bearers for `ReportPolicyStatus` and `GetSandboxProviderEnvironment`,
matching upstream sandbox-only auth mode. `ReportPolicyStatus` is bound to the
authenticated sandbox generation and epoch by its handler.

## Latest local acceptance and hosted runner recheck

- Rechecked: 2026-10-22 17:15 Europe/Samara.
- `task test` passed across all Go workspace modules after Gateway auth, policy
  status and log delivery changes. `task vm:live-darwin` passed on Apple
  Silicon/HVF (21.94s), `task vm:parity-check` passed for all 30 inventory
  entries, and focused Gateway/SDK/CLI packages passed.
- Hosted workflow run
  [39764654224](https://github.com/KLORQ/quarshell/actions/runs/39764654224)
  was queried with `gh`: Linux KVM, macOS HVF and Windows MXC jobs all failed
  before steps; GitHub reports empty step lists. This is consistent with the
  account billing/spending-limit failure already recorded in the plan, so it
  provides no new hardware acceptance evidence.
- Current inventory at that verification was 4 implemented, 24 partial, 2
  unsupported and 0 open. Full VM migration is the declared priority;
  hardware-specific KVM and GPU/VFIO acceptance and credential-bound provider
  environment remain open.

## Sandbox process-exit RPC implementation

- Implemented and verified: 2026-10-22 17:23 Europe/Samara.
- `ReportMainProcessExit` and `FinalizeMainProcessExit` now persist exit status,
  terminal sandbox state and finalization in the lifecycle store. Reports bind
  to the authenticated sandbox, runtime generation/credential epoch and the
  active supervisor instance; retries retain the first process result.
- The supervisor lifecycle bridge now sends its Sandbox Protocol bearer for
  these RPCs. The unary interceptor accepts Sandbox audience for process exit,
  policy acknowledgement, provider environment and other sandbox-only methods;
  Gateway audience remains for token refresh.
- `go test -C quarshell-gateway ./internal/application/sandbox
  ./internal/transport/applicationgrpc -count=1` and
  `go test -C quarshell-runtime ./relayclient ./internal/app/supervisor
  -count=1` pass. Added tests cover first-result idempotency, finalization,
  restart reset, stale generation, wrong sandbox/instance and auth audience.
- This closes the terminal process result portion of Gateway running-boundary
  parity. Provider environment, sandbox watch/durable logs, shared HA leases,
  credential rotation/replay and hardware acceptance remain open.
- Full `task test` passes across every Go workspace module, and
  `task vm:live-darwin` passes on Apple Silicon/HVF (21.99s) after switching
  the lifecycle bridge to the Sandbox bearer. The live acceptance exercised
  boot/recovery, policy revision ACK, natural process exit, explicit
  stop/start, registry authentication and paired supervisor credentials.

## SQLite-backed bounded supervisor log window

- Implemented and verified: 2026-10-22 17:31 Europe/Samara.
- Supervisor log batches are atomically stored per sandbox through the shared
  lifecycle store, retaining the newest 2000 lines up to 8 MiB. With Gateway
  `--data` set to SQLite, the window survives closing/reopening the Gateway
  store. `GetSandboxLogs` reads the durable window; in-memory retention remains
  only as the no-store test/embedder fallback.
- Tests reopen a SQLite store after 2001 events and verify the oldest line was
  evicted, newest retained, and sandbox/message bounds are enforced. gRPC log
  ingest/query tests exercise the durable service integration.
- `WatchSandbox`, resumable cursor semantics, shared multi-replica event
  delivery, endpoint-bound provider credentials and hardware-specific KVM/GPU
  remain open.

## WatchSandbox log/status slice

- Implemented: 2026-10-22 17:38 Europe/Samara.
- Gateway `WatchSandbox` now sends the initial sandbox snapshot, replays the
  requested bounded log tail (upstream default 200), filters by source, minimum
  level and timestamp, streams matching live lines and changed status
  snapshots, and honors `stop_on_terminal`. Durable local log cursors support replay within
  the current Gateway epoch and report expired/trimmed cursors. Watchers are
  removed on stream end and validation errors; replay/live overlap is
  deduplicated.
- Workspace `task test`, `task vm:parity-check` (30 entries), `task fmt:check`
  and `git diff --check` passed after the first WatchSandbox parity fixes.
  Focused `TestWatchSandbox` tests passed after follow-up fixes for initial
  snapshot parity, watcher cleanup, deduplication, default tail and timestamp
  filtering. The full workspace suite has not been rerun after those final
  edits.
- This is a partial match for OpenShell `WatchSandbox`: platform-event replay,
  source-merged resumable cursor semantics and cross-replica delivery are still
  missing. The VM driver's `watch_sandboxes` platform lifecycle feed also
  remains incomplete, so the `vm.logs-watch` parity row stays partial.
- Final verification after these edits: `task test`, `task vm:parity-check`
  (30 pinned rows), `task fmt:check`, `git diff --check`, focused Gateway
  WatchSandbox tests and `task vm:live-darwin` all passed. Live HVF acceptance
  completed in 24.22s and exercised policy revision ACK, Sandbox-bearer
  process/log delivery and natural workload exit.
- A follow-up comparison change makes `follow_status` emit any changed sandbox
  snapshot, not only a phase transition; snapshots now include the canonical
  main-process instance ID and exit code from the durable process report.
  Focused Gateway tests and `git diff --check` passed after that change; full
  workspace gates have not been rerun since.
- The existing provider binding RPCs now verify the provider definition,
  distinguish a new attachment from an already bound provider, and propagate
  binding-store errors during detach. Provider refresh configuration rejects
  stale credential keys and invalid expiry timestamps, captures the returned
  value while holding the mutex, and has focused regression coverage. This
  remains partial: resource-version receipts, durable secret storage,
  endpoint-bound provider environment materialization and supervisor install
  are not complete. Workspace and HVF checks above preceded these provider
  lifecycle edits; focused provider/rootfs tests passed afterward.

## Latest local verification after Gateway watch/provider fixes

- Rechecked: 2026-10-22 17:49 Europe/Samara.
- `task test`, `task vm:parity-check` (30 pinned OpenShell v0.1.4 rows),
  `task fmt:check` and `git diff --check` passed after the sandbox status
  projection, provider binding and provider credential validation changes.
- `task vm:live-darwin` passed on Apple Silicon/HVF in 23.07s, including VM
  recovery, policy ACK/revocation, sandbox bearer auth, supervisor process
  reporting and natural workload exit. The latest provider edits are covered
  by the workspace and focused Gateway tests; they do not alter the VM driver.
- Inventory remains 4 implemented, 24 partial, 2 unsupported (GPU/VFIO),
  0 blocked and 0 open. GitHub-hosted workflows still fail before job steps,
  so they contribute no new acceptance evidence.
- Additional VM deployment gates passed locally: `task vm:artifact-check`
  verified all 7 runtime artifacts and digest failures, `task vm:install-check`
  passed install/upgrade/rollback, and `task vm:kvm-script-check` passed its
  fixture lifecycle checks without requiring `/dev/kvm`.
- VM, Gateway and runtime cross-builds passed for Linux ARM64 and Windows
  AMD64 (`CGO_ENABLED=0`). Gateway cross-builds require the repository
  `go.work`; the initial invocation without `GOWORK` failed to resolve that
  workspace and was repeated successfully with it configured.

## Provider environment scope and secret withholding

- Sandbox-scoped runtime reports now require the authenticated Sandbox bearer
  to match the requested sandbox and its active runtime generation and
  credential epoch. This check is shared by provider environment, provider
  readiness, endpoint status and sandbox configuration report handlers.
- Until provider profile/policy resolution can produce endpoint bindings,
  `GetSandboxProviderEnvironment` returns no credential values and sets
  `CREDENTIALS_WITHHELD` when an attached provider has an unexpired secret.
  This prevents a provider token from becoming unrestricted workload
  environment. Durable provider credentials remain encrypted at rest.
- Focused tests verify cross-sandbox and stale-generation rejection and that
  attached unbound credentials are not returned. Full workspace gates are
  running after this correction.
- Full `task test`, `task vm:parity-check` (30 pinned rows), `task fmt:check`
  and `git diff --check` passed after this correction. The subject-token
  exchange fixture now carries the active sandbox principal and generation,
  matching the enforced runtime scope.

## GitHub-hosted recheck for the migration commit

- Commit `de12743b97832bb5d89f780a7ce141711bc1de79` secret scan passed in
  [run 39791083670](https://github.com/KLORQ/quarshell/actions/runs/39791083670).
- The [platform matrix 39791077231](https://github.com/KLORQ/quarshell/actions/runs/39791077231)
  completed successfully: macOS/HVF and Linux/KVM VM packages built and
  tested; Windows VM/driver build and Windows-compatible package tests passed.
  Its live VM checks were skipped because hosted macOS 14 lacks
  Hypervisor.framework and hosted Ubuntu has no usable non-root `/dev/kvm`.
  Windows runner build 26100 also lacks MXC runtime files and is below the
  required build 26300, so native MXC remains unverified.
- The [macOS workflow 39791070695](https://github.com/KLORQ/quarshell/actions/runs/39791070695)
  passed. The [Linux workflow 39791064035](https://github.com/KLORQ/quarshell/actions/runs/39791064035)
  passed workspace tests/policy conformance, Docker and rootful/rootless
  Podman acceptance, Gateway image/Compose install and docs.
- Security follow-up commit `f9438a1` is pushed; its secret scan passed in
  [run 39791817481](https://github.com/KLORQ/quarshell/actions/runs/39791817481).
  Its macOS workflow passed in
  [run 39791817531](https://github.com/KLORQ/quarshell/actions/runs/39791817531).
  Linux workflow [39791817542](https://github.com/KLORQ/quarshell/actions/runs/39791817542)
  passed workspace tests, Docker/Podman E2E, docs and Gateway image/Compose;
  the overall run completed successfully.

## VM lifecycle events in WatchSandbox

- VM `WatchPlatformEvents` subscribes before loading its initial snapshot,
  then receives lifecycle updates directly from successful persisted state
  writes and deletions. It emits state changes and a terminal deletion event;
  the stream is context-cancelable and its bounded process-local channel uses
  best-effort overflow behavior like the pinned upstream broadcast feed.
- The Gateway runtime adapter exposes the optional driver event stream.
  `WatchSandbox(FollowEvents)` now accepts event-only watches and forwards
  events after its initial sandbox snapshot; sandbox ID and `since_time` are
  checked. Focused VM watch and Gateway WatchSandbox event tests passed.
- This closes the missing VM lifecycle event source and its Gateway delivery
  path. At this stage the initial event path did not yet retain a shared cursor;
  the merged log/event replay implementation and current limitations are
  recorded in the following section. The `vm.logs-watch` inventory row
  correctly remains partial.
- After this change, `task vm:live-darwin` passed again on local Apple
  Silicon/HVF (`TestLiveMicroVMLifecycle`, 22.09s test time). The test now
  subscribes before VM start and observes real `created`, `running`, and
  terminal `stopped` events from the VM state store. Full workspace tests,
  focused Gateway watch tests, formatting, and parity inventory checks passed
  on the same tree.

## Merged log and platform-event replay

- The SQLite-backed sandbox log window now stores bounded platform-event
  records in the same per-sandbox sequence space as supervisor logs. Both
  sources receive opaque Gateway cursors; repeated VM snapshots are
  deduplicated, and retention tracks independent log/event trim watermarks.
- `WatchSandbox` supports `event_tail`, resumes both sources after one cursor,
  sorts replay by shared sequence, and performs an atomic live catch-up from a
  single durable snapshot. Filtered log records advance their source watermark
  to suppress duplicate delivery. Cursor-ahead and source-trimmed requests
  fail explicitly. Live merge tests cover interleaved replay and a terminal
  event arriving alongside a later log line.
- `sandboxlogs` tests cover SQLite reopen with shared sequence numbers,
  event deduplication, and event-window trimming. Gateway tests cover event
  tail, cross-source replay/resume, live merge ordering, and terminal stop.
- Remaining limits: platform history is collected when a Gateway
  `WatchSandbox` subscription observes events; the VM state feed is
  process-local/best-effort; cursor epochs reset when Gateway restarts; and
  multi-replica live event delivery is not shared. The Gateway composition
  watch bridge still lacks a live HVF composition test.
- On the current tree, full workspace `task test`, focused race tests for
  sandbox log persistence and Gateway watches, `task vm:parity-check`,
  `task fmt:check`, and `git diff --check` passed. VM package test binaries
  cross-compiled for Linux ARM64 and Windows AMD64; Gateway WatchSandbox tests
  cross-compiled for both targets with CGO disabled.

## Current-tree Mac rerun after merged WatchSandbox cursors

- Time: 2026-10-22 18:37 Europe/Samara; Apple Silicon, macOS ARM64, HVF/libkrun.
- `task vm:live-darwin`: PASS (`TestLiveMicroVMLifecycle`, 23.98s). The live
  VM platform watch observed `created`, `running`, and terminal `stopped`;
  policy revision, worker recovery, stop/start, broker-loss shutdown, OCI
  image pinning, and Gateway credential exchange also passed.
- `task vm:parity-check`: PASS (30 pinned OpenShell v0.1.4 entries);
  `task fmt:check` and `git diff --check`: PASS.
- This confirms the VM backend after the recent watch work. It does not exercise
  the live Gateway `WatchSandbox` composition bridge or turn the partial
  `vm.logs-watch` row into full parity.

## Driver-wide VM sandbox watch

The VM driver now exposes an optional `SandboxWatchStream` contract. Its
`WatchSandboxes` feed emits a snapshot for each persisted VM, then state updates
and deletion IDs from successful state-store mutations. Subscription is
registered before the snapshot read to avoid missing transitions during
startup. The feed uses a bounded 128-event process-local queue and does not
claim cross-process replay. `TestVMWatchSandboxesSnapshotsUpdatesAndDeletion`
covers initial snapshots, state change, deletion, and cancellation. The
`vm.logs-watch` parity row remains partial because Gateway composition and
process-independent replay are still missing.

After this addition, the full workspace `task test`, `task vm:parity-check`,
`task fmt:check`, and `git diff --check` passed. Focused VM watch tests passed
under `-race`; VM test binaries cross-compiled for Linux ARM64 and Windows
AMD64 with CGO disabled. The newest `task vm:live-darwin` rerun passed on
Apple Silicon/HVF at 18:42 (22.55s), again observing lifecycle events and
completing the credential/session acceptance.

The inventory also marks `vm.resource-capabilities` implemented for the
default configuration: the VM reports CPU/memory limit overrides unsupported
and GPU selection disabled, matching the pinned VM driver's default capability
response. `TestBackendCapabilitiesAdvertiseRuntimeExtensions` now asserts all
three resource capability fields and the disabled GPU selection flags. The
GPU/VFIO inventory and crash-recovery entries remain unsupported without GPU
implementation and hardware.

## Durable WatchSandbox cursor and shared-store catch-up

The Gateway stores its opaque cursor epoch in the lifecycle store and loads it
before processing a `WatchSandbox` request. Reopening SQLite preserves that
epoch, so a cursor issued before restart remains valid when the same retained
window is available. Watchers poll the durable log/event window every second
and merge records newer than their per-source watermarks; this delivered a log
written through a second Gateway server in the regression test. The platform
event producer is still process-local: a new VM transition must first reach a
local `WatchPlatformEvents` subscriber to be appended. The cross-instance test
also seeds a terminal platform event and a later log from the shared store and
checks ordered delivery plus `stop_on_terminal` behavior.

Validation on the current tree at 2026-10-22 20:11 Europe/Samara: full
workspace `task test`, `task vm:parity-check`, `task fmt:check`, and
`git diff --check` passed. Focused cursor, Gateway watch and sandboxlogs tests
also passed under `-race`; Gateway WatchSandbox test binaries cross-compiled
for Linux ARM64 and Windows AMD64 with CGO disabled. The inventory is 6
implemented, 22 partial, and 2 hardware-specific unsupported.
Concurrent SQLite cursor epoch initialization passed with eight callers under
`-race`; all instances observed the same durable cursor namespace.

## VM state watch across processes

At 2026-10-22 20:15 Europe/Samara, VM per-sandbox platform-event watches and
driver-wide sandbox watches were extended to poll the persisted state directory
in addition to receiving immediate local notifications. A second process using
the same state directory is now observed without a local in-memory publisher;
updates and deletions are covered by direct-file-write regression tests. The
poll intervals are 250 ms and 1 second respectively, so short intermediate
states can coalesce and the feed still does not offer durable event replay.
Focused VM watch tests passed under `-race`.

`task vm:live-darwin` passed on Apple Silicon/HVF at 20:15 (26.21 s). It
observed `created`, `running`, and `stopped`, verified denied raw ICMP and
unmapped vsock, allowed a guest request through the trusted HTTPS CONNECT
upstream after policy revision 2 (HTTP 200), revoked it after revision 3
(HTTP 403), recovered and restarted workers, pinned OCI image content, and
completed Gateway credential and process-event acceptance. This live test
does not exercise the Gateway `WatchSandbox` composition bridge. The pinned VM
parity inventory remains 6 implemented, 22 partial, and 2 hardware-specific
GPU/VFIO unsupported.

## Live TLS workload egress over HTTPS CONNECT

At 2026-10-22 20:22 Europe/Samara, `task vm:live-darwin` passed again
(`TestLiveMicroVMLifecycle`, 23.92 s) with the policy egress probe upgraded
from HTTP to an actual HTTPS workload request. The local HTTPS upstream proxy
now establishes CONNECT and tunnels bytes to a local TLS workload server;
the host policy proxy trusts the fixture certificates through its configured
egress CA bundle. Revision 2 allowed `example.com:443`, and the guest received
HTTP 200 over TLS. Revision 3 denied the CONNECT (`Forbidden`) before it reached
the TLS workload. The rest of the live VM, recovery, OCI pinning, and Gateway
credential/process-event flow also passed. This supplies live TLS workload
egress evidence, while the broader upstream corporate-proxy parity remains
partial (credentialed proxy auth, `no_proxy` bypass and the full upstream
fixture matrix are not covered by this test). The temporary guest probe skips
destination certificate verification so this test isolates TLS transport and
policy routing; it does not establish the guest's trust of the proxy's
ephemeral MITM CA.

The same current source passed `task vm:live-darwin` again at 20:30
(`TestLiveMicroVMLifecycle`, 22.61 s), confirming the checked-in probe and the
certificate-verification behavior used by the pinned upstream fixture.

At 20:33, the live fixture was extended to require upstream proxy Basic auth
read from a mode-0600 credential file. The local HTTPS CONNECT proxy rejected
requests without the expected `Proxy-Authorization`; `task vm:live-darwin`
passed again (`TestLiveMicroVMLifecycle`, 22.81 s), including authenticated
TLS egress allow/revoke and the full VM/Gateway lifecycle path. This covers the
credentialed HTTPS proxy leg of upstream parity; `no_proxy` direct bypass and
the complete upstream fixture matrix remain untested.

At 20:35, the fixture also required the upstream CONNECT target to be a
resolved IP on port 443, used the upstream policy's explicit `tls: skip`
setting, and asserted a revoked request never reached the upstream proxy.
`task vm:live-darwin` passed again (`TestLiveMicroVMLifecycle`, 22.68 s).
Together with the proxy-layer exact host/port `no_proxy` direct-dial test and
auth/CA tests, this closes the pinned `vm.policy-egress` row for supported
configuration. The guest HTTPS probe skips server certificate verification,
matching the upstream fixture; this row does not establish guest trust of the
ephemeral MITM CA.

## Working upstream proxy in live HVF acceptance

At 2026-10-22 20:07 Europe/Samara, `task vm:live-darwin` passed on Apple
Silicon/HVF (`TestLiveMicroVMLifecycle`, 23.81s). The test starts a local TLS
HTTP CONNECT proxy whose certificate is trusted through the configured proxy
CA bundle. A guest request to the previously blocked `policy-probe.invalid`
endpoint succeeds with HTTP 200 after policy revision 2 allows it; revision 3
then revokes it with HTTP 403. The test verifies both certificates are installed
in the guest trust bundle. This covers the host policy proxy, its HTTPS upstream
TLS trust and the guest CA installation end to end; it does not replace a real
external corporate proxy deployment or the live Gateway `WatchSandbox`
composition test.

This acceptance closes the `vm.corporate-proxy-ca` inventory row for the pinned
default configuration. `vm.policy-egress` remains partial because its upstream
parity target also requires approved TLS workload egress, beyond the HTTP guest
probe used here.

## Interactive non-TTY VM exec

- Reverified: 2026-10-22 22:24 Europe/Samara
- Linux guest-agent test: passed in a local `alpine:3.22` container.
- Live host-to-guest acceptance: `task vm:live-darwin` PASS; `TestLiveMicroVMLifecycle` PASS (23.09 s), local Docker OCI import PASS (1.18 s).
- Gateway test: non-TTY interactive stdin, EOF, separate stdout/stderr, exit code, and completed request-ID duplicate suppression pass under race.

The guest accepts an authenticated `exec-stream` control operation, starts a non-PTY process group, streams stdout and stderr separately, forwards stdin, half-closes on EOF, kills the process group if the transport is lost, and sends the normalized exit code. The live HVF acceptance verified those operations against the Linux guest. `vm.exec-pty-signals` remains partial for full upstream PTY signal behavior.

## Exact-current-HEAD Mac retest

- Commit: `d1b904debc9ceb2066905c0f92229a769005a24d`
- Reverified: 2026-10-22 22:28 Europe/Samara
- `task vm:live-darwin`: PASS; `TestLiveMicroVMLifecycle` (22.83 s) and
  `TestLiveLocalOCIEngineImport` (1.21 s). The real HVF run observed
  created/running/stopped, worker recovery, policy allow/revoke through
  authenticated HTTPS CONNECT, gateway credential exchange, non-TTY
  stdin/stdout/stderr/exit, terminal interrupt, image pinning, and broker-loss
  shutdown.
- `task test`: PASS across all workspace Go modules.
- `task vm:parity-check`: PASS (30 pinned entries); `task fmt:check` and
  `git diff --check`: PASS.
- The Gateway-composition WatchSandbox live probe and full upstream running
  boundary, session rotation/replay, KVM live lane, and physical GPU/VFIO remain
  open. Hosted workflows had not yet been dispatched for this exact commit at
  the time of this record.

## Exec stream transport-loss regression

After the `d1b904d` live retest, the Linux guest-agent suite gained
`TestGuestControlExecStreamTerminatesProcessGroupOnDisconnect`. A cross-built
Linux ARM64 test binary ran in local `alpine:3.22`; the test passed ten
consecutive times. It confirmed transport loss terminates the command process
group and exits the stream handler. Linux process zombies are considered
terminated because minimal container init processes may not reap them promptly.

## VM CopyTo source handling

`CopyTo` now uses a no-follow descriptor open on macOS/Linux, checks type and
initial size on that descriptor, and caps bytes read to the maximum control
frame payload. The non-Unix implementation also bounds the read. The focused
test rejects a final symlink and oversized source and preserves regular-file
permissions. Full `task test`, parity, format/diff checks, and Windows VM test
binary cross-compilation passed. The subsequent `task vm:live-darwin` passed
(`TestLiveMicroVMLifecycle`, 22.87 s; local OCI import, 1.39 s), including the
existing live file copy round-trip. Directory/archive transfer through the
complete live VM+Gateway CLI path is not yet covered.
