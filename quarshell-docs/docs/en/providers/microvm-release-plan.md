---
status: draft
---

# MicroVM parity, production, and release plan

This plan takes the current libkrun MicroVM path to OpenShell v0.1.4 functional
parity and a production release. It supplements the
[MicroVM guide](microvm.md) and the [runtime acceptance matrix](../reference/runtime-acceptance.md).
Change an item’s status only after implementation and target-host verification.

## Current state

The local driver can create and recover VMs, import OCI rootfs and tar, keep a
persistent overlay, run commands and PTY sessions, stream logs, copy files,
proxy policy traffic, and relay SSH direct-tcpip. The two-disk
bootstrap/workload path is implemented: explicit `OPENSHELL_VM_BOOTSTRAP_IMAGE`
or trusted `OPENSHELL_SANDBOX_IMAGE` fallback for bootstrap, separate
read-only workload disk, and persistent overlay. Gateway currently blocks VM
creation until host supervisor/isolation and acceptance gates pass.

Open parity inventory items include:

- `vm.bootstrap-image`: two-disk boot and digest recovery pass macOS acceptance,
  guest init now has `image-prep` branches for versioned tar and OCI layout
  payloads. The host stages tar/OCI layout, launches the prep worker on the
  pinned bootstrap, verifies ext4 with debugfs, then publishes the workload
  disk. Live macOS acceptance passes; status stays `partial` until Linux KVM
  and the full failure/security matrix pass.
- `vm.gateway-create-ready`: end-to-end Gateway create/start/ready is blocked.
- Linux KVM amd64/arm64, installed distribution/upgrade/rollback, and GPU/VFIO
  require target hosts; GPU/VFIO and PID limits are currently unsupported.
- Other open or partial `vm.*` items are tracked in
  `quarshell-driver-vm/vm/testdata/openshell-v0.1.4-parity.json`.

## Phase 1. Freeze the contract and parity

- [x] Compare each `vm.*` ID with upstream v0.1.4 and record minimum semantics
  for create/start/stop/delete, recovery, overlay, identity, bootstrap, OCI/tar,
  exec/PTY, logs/events, SSH forwarding, and GPU/resource capabilities.
- [x] Define separate bootstrap and workload image configuration and precedence:
  bootstrap image provides system agent/supervisor; workload image provides the
  user rootfs. Never silently use the workload image as bootstrap fallback.
- [x] Port the OpenShell two-disk design: bootstrap root disk, separate
  read-only workload image disk mounted at `/image-rootfs`, and a persistent
  writable sandbox overlay. Prepare images in a bootstrap prep VM and validate
  `/image-rootfs` plus the guest agent digest before publishing the disk into
  the immutable cache. Live macOS ARM64/HVF acceptance passes; Linux KVM
  acceptance remains a separate release gate.
- [x] Persist bootstrap/workload digests and references to both immutable cache
  entries; recovery must validate both artifacts, hold both cache leases, and
  never pull a mutable tag as a replacement.
- [x] Verify bootstrap/workload/overlay disk ordering in the fake-worker test;
  live OCI acceptance confirms the workload image mount, writable persistent
  `/sandbox` overlay across Stop/Start, and recovery from the same pinned digest
  after the image tag moves. Unit tests reject missing, truncated, and corrupt
  ext4 workload disks. Linux KVM acceptance remains a required separate gate.
- [ ] Complete lifecycle extension parity. Implemented and tested: the registry,
  deterministic ordering and reverse cleanup, activation only through
  registered trusted labels, descriptor/launch-plan and required backend feature
  validation (guest-init-dropins is supported; unsupported features fail closed),
  private extension state, runtime-only environment,
  ConfigureLaunch/BeforeLaunch/BeforeRestore/AfterRestore/Stopped/ProcessExited/
  AfterDelete hooks, and executable guest-init drop-ins in the overlay. Live
  macOS ARM64 acceptance verifies label activation and drop-in execution.
  Restore ConfigureLaunch/digest/BeforeRestore failures now run reverse
  extension cleanup with `restore-failed`; AfterRestore failures also trigger
  cleanup. Regression unit tests cover both paths. Broader backend capability
  resolution and Linux KVM failure-path verification remain open.
- [x] Cover trusted bootstrap fallback, rejection of an incompatible OCI
  platform, recovery by digest after a tag move, and excluding environment and
  credentials from durable state (`TestDefaultHostConfigUsesTrustedDefaultImageAsBootstrapFallback`,
  `TestOCIImageIndexRejectsMissingPlatformDescriptor`, live OCI acceptance,
  `TestCreateWithBootstrapImagePersistsSeparatePinnedDisks`).
- [x] Preserve OpenShell semantics when `rootfs_tar_path` and `image` are both
  supplied: the tar provides the workload filesystem while the image reference
  remains the requested identity. `TestValidateSpecAllowsRootfsTarWithImageSelector`,
  Create unit coverage, and live acceptance exercise the path; earlier GPU
  rejection retains precedence.
- [x] Update the parity JSON with unit and live acceptance evidence; keep entries
  `partial` until the remaining parity and target gates pass.

Target configuration shape after implementation:

```go
type MicroVMImages struct {
    Bootstrap OCIImageRef // pinned linux/amd64 or linux/arm64
    Workload  OCIImageRef // sandbox image; independent from bootstrap
}

func (c MicroVMImages) Validate() error {
    if c.Bootstrap.Digest == "" {
        return errors.New("MicroVM bootstrap image must be pinned by digest")
    }
    if c.Workload.Digest == "" {
        return errors.New("MicroVM workload image must be pinned by digest")
    }
    return nil
}
```

This is a target API shape, not a claim about current implementation. First
reconcile it with existing `driver_config`, Gateway API, and upstream behavior.

### P0: bootstrap prep VM (first required vertical slice)

Complete workload preparation before Gateway work. Today `vm.go` calls
host-side `prepareRootfsTarWorkloadDisk` / `prepareOCIWorkloadDiskFromResolved`,
and `populateRootfsFromOCI` applies OCI layers on the host. Digest verification
is present, but preparation does not yet use the upstream VM boundary.

Keep staging, guest execution, and cache publication as separate operations:

```go
type PrepInput struct {
    BootstrapDisk string // immutable, digest pinned
    PayloadDisk   string // OCI layout or rootfs tar; no registry credentials
    OutputDisk    string // temporary ext4 for this invocation
    SourceDigest  string
    Platform      string // linux/amd64 or linux/arm64
}

func prepareWorkloadInGuest(ctx context.Context, in PrepInput) (string, error) {
    if err := validatePrepInput(in); err != nil { return "", err }
    if err := runPrepWorker(ctx, in, "QUARSHELL_VM_INIT_MODE=image-prep"); err != nil {
        return "", err
    }
    if err := verifyExt4Directory(in.OutputDisk, "/image-rootfs"); err != nil {
        return "", fmt.Errorf("guest image-prep did not produce image-rootfs: %w", err)
    }
    return publishImmutableCache(ctx, in.OutputDisk, in.SourceDigest, in.Platform)
}
```

Implementation and verification order:

1. Add a versioned payload format for tar and verified OCI layout. Registry auth
   stays on the host during pull; payloads contain blobs/manifest and source
   identity only, with no environment, token, or credential-helper data.
2. Add guest init `image-prep`: mount payload read-only and output writable,
   unpack into `image-rootfs.partial`, apply ownership/runtime setup, atomically
   rename to `image-rootfs`, sync, unmount, and exit.
3. Launch a dedicated libkrun worker on the same pinned bootstrap disk; pass the
   mode through allowlisted environment. Handle cancellation, timeout, console
   tail, process-group cleanup, and staging removal on every failure.
4. Do not trust exit status alone: validate ext4, required files, metadata, and
   source identity before atomic cache publication. Persist the workload digest
   only after publication succeeds.
5. Include payload format and guest init digests in the cache key; use cache
   locks/leases and verify publication under concurrent preparation.

Minimum test examples:

```go
func TestPrepPublishesOnlyVerifiedImageRootfs(t *testing.T) { /* fake worker + ext4 verifier */ }
func TestPrepCancellationRemovesStagingAndDoesNotCache(t *testing.T) { /* cancel worker */ }
func TestPrepPayloadNeverContainsRegistryCredentials(t *testing.T) { /* scan staged payload */ }
```

Run unit/fake-worker tests on every PR, then live acceptance on macOS
ARM64/HVF and Linux KVM amd64/arm64. Keep `vm.bootstrap-image` `partial` and the
release gate closed until every target lane passes.

## Phase 2. Complete host supervisor and isolation

- [ ] Launch Gateway-managed workers through a host supervisor with a private,
  authenticated control channel, limited RPC surface, and managed shutdown.
  The VM and image-prep workers now receive a private parent-liveness pipe and
  kill their process group, including the broker, when the Gateway exits. This
  closes the orphan-worker path, but does not yet provide a separate managed
  supervisor process or host filesystem isolation.
- [ ] On Linux, establish private user/mount namespaces, private mount
  propagation, a dedicated non-root host identity, and explicit UID/GID maps;
  verify them before starting the broker or VM.
- [ ] On macOS ARM64, launch a signed and verified worker bundle through a
  separate supervisor path. Gateway preflight now requires
  `QUARSHELL_VM_WORKER_TEAM_ID`, verifies the code signature with
  `codesign --verify --strict`, and matches the Team ID; the managed supervisor
  and live Gateway acceptance are still unimplemented.
- [ ] Do not pass host workspace mounts. Keep workspace in a persistent guest
  overlay and test host filesystem/network/vsock isolation.
- [ ] Provide token revocation, rotation/retry, process-group cleanup, and no
  resurrection after terminal workload exit.
- [ ] Keep Gateway fail-closed until the supervisor and both target platform
  acceptance lanes pass.

Fail-closed capability check before enabling the provider:

```go
if !hostIsolation.Ready() || !hostSupervisor.Ready() {
    return status.Error(codes.FailedPrecondition,
        "MicroVM host isolation is not ready")
}
```

## Phase 3. Gateway end-to-end path

- [ ] Implement Gateway create → driver create → start → guest readiness →
  Get/List status with observed workload identity.
- [ ] Test create cancellation, timeout, client disconnect, and cleanup at every
  transition.
- [ ] Test stop/delete, Gateway restart with running/stopped intent, worker or
  broker loss, retry Start, and terminal exit.
- [ ] Test extension activation only after policy-controlled labels, rejection
  of unknown extensions, and cleanup of extension state on delete.
- [ ] Test `WatchSandbox` log/event tail replay and stream shutdown.
- [ ] Test interactive exec stdin/output/resize/signals/exit status, copy-in/out,
  policy proxy, and SSH TCP forwarding through public CLI/API.
- [ ] Test rootfs upload limits, OCI auth, digest pinning, and recovery failure
  after a tag moves.

## Phase 4. Pre-release test matrix

**Every change:**

```sh
cd quarshell-driver && go test ./...
cd ../quarshell-gateway && go test ./...
cd ../quarshell-runtime && go test ./...
cd .. && task vm:parity-check compat:inventory docs:strict
```

**Local live acceptance on macOS Apple Silicon/HVF:**

```sh
task vm:live-darwin
```

This exercises the real worker/guest path, lifecycle/recovery, OCI digest/auth,
PTY, identity, policy, and SSH TCP roundtrip. A macOS pass does not replace
Linux KVM or Gateway acceptance.

**Required target CI runners:**

```sh
# Linux amd64 and arm64 runners with /dev/kvm and configured runtime/rootfs
task vm:live-linux

# Public Gateway/CLI end-to-end test with the VM provider
QUARSHELL_VM_GATEWAY_ACCEPTANCE=1 task vm:gateway-acceptance
```

`vm:live-linux` runs the existing KVM acceptance script for amd64/arm64.
`quarshell-driver/.github/workflows/live-vm-kvm.yml` now runs both native KVM lanes
on pushes and pull requests; configure both jobs as required branch checks and
provide self-hosted runners labeled `quarshell-kvm`. The Gateway task
`vm:gateway-acceptance` does not exist yet and must be implemented and added to
CI. Required gates must fail rather than skip when host capabilities are
missing. All driver test packages locally cross-compiled for `linux/amd64` and
`linux/arm64` (`GOOS=linux GOARCH=... CGO_ENABLED=0 go test -run '^$' -exec
/usr/bin/true ./...`). This confirms Linux build paths only; it does not run
tests or replace KVM acceptance.

Run resource and failure checks for cache exhaustion during create/delete,
CPU/RAM limits, restart after worker kill, corrupted state, missing KVM/HVF,
registry outage, invalid digest, and namespace failure. Run GPU/VFIO only on a
dedicated compatible runner; until then advertise the capability as unsupported.

## Phase 5. Install, upgrade, and rollback

- [ ] Build separate signed artifacts for Linux amd64/arm64 and macOS ARM64;
  pin libkrun, guest agent, worker, and state layout versions.
- [ ] Publish checksums, SBOM, provenance, and signature verification steps.
- [ ] Test clean installation without local Go replacements or unpublished
  dependencies, then upgrade from the previous stable version.
- [ ] Test state migration on copied data, incompatible state versions,
  interrupted migration, and rollback. Migration must be atomic or retain a
  recoverable backup.
- [ ] Test install/upgrade/uninstall on clean Linux amd64/arm64 and macOS ARM64
  hosts; uninstall must not remove user VM data without an explicit operator
  action.
- [ ] Release a canary, inspect startup/isolation/recovery errors, then promote
  the same artifact digest to stable.

## Release criteria

Release when required parity IDs are implemented or explicitly excluded from
the product profile; Gateway create-ready and supervisor/isolation pass
acceptance on macOS ARM64 and Linux KVM amd64/arm64; install/upgrade/rollback
pass on published artifacts; and tests, security checks, docs, and parity
inventory are green. GPU/VFIO may be excluded from the first profile only if
the product does not claim GPU support and does not advertise that capability.
