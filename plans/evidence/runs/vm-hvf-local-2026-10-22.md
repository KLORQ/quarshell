# VM Hypervisor.framework local acceptance

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

The gap remains real: `validateGatewayRuntime` always returns a not-ready
error, and production call sites do not instantiate `isolation.Registry` or
attach a backend. The existing gate release is driver-local and is not tied to
the `VerifiedDescriptor`/`EnforcementEvidence` contract. Therefore Gateway
VM creation remains fail-closed; direct HVF lifecycle results do not close G04.
