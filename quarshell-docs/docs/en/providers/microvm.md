---
status: soon
---

<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# MicroVM

<div class="ws-soon" markdown>

![Under construction](../../assets/images/under-construction.svg){ width="420" }

**Experimental.** The local driver works, but Gateway use is not enabled yet.

</div>

## Current implementation

The driver creates one libkrun VM per sandbox. It can import a Linux rootfs from
an OCI registry reference or a staged tar archive, build an immutable ext4 base
image, and give each sandbox a writable overlay. Registry references use the
host Docker/Podman credential keychain, including Docker credential helpers. The selected image platform is
`linux/<host-architecture>`; manifest and layer digests are checked before the
rootfs enters the cache. Layer application preserves setuid, setgid and sticky
permission bits in addition to regular permissions.

The driver also has an authenticated guest control channel using versioned v1
JSON over length-prefixed frames; host and guest reject missing or unsupported
versions. After the guest hardening step drops workload privileges, `cautem-init`
reports its effective UID:GID through the supervisor readiness handshake. The
VM driver persists that observed identity and Gateway Get/List responses expose
it as the OpenShell `WorkloadIdentity` condition. `start-workload` returns only after the guest agent observes a private,
versioned readiness marker written by the supervisor after it starts the
workload. The marker path is removed from the child workload environment.
The host rotates the per-sandbox token on each successful Start or recovery.
Rotation state is kept in the private sandbox directory, and a retry after a
lost response reuses the same rotation ID. The channel also provides process
restart, exec, logs, file copy, policy proxy and SSH relay. The SSH relay carries
OpenSSH `direct-tcpip` channels to guest loopback; macOS ARM64 live acceptance
round-trips data through a TCP server inside the guest.
Exec supports bounded one-shot commands and a full-duplex PTY session with
stdin, window resize and terminal-generated signals. The Gateway compatibility
adapter has a separate VM interactive-exec path. Its VM workspace request uses
the persistent guest overlay and does not pass a host directory. Gateway
sandbox creation still fails closed until the host supervisor/isolation
contract and target-host gates pass.
The image rootfs is cached under the VM state directory. The default cache
quota is 64 GiB; unreferenced images are reclaimed when more space is needed.

## Production readiness

The Linux worker now starts in private user and mount namespaces and makes mount
propagation private before creating its broker or VM. Startup fails when those
namespaces cannot be established or when the Gateway itself runs as host root.
The worker's namespace root still maps to the Gateway account; this is not a
dedicated host UID. macOS launches a separate worker process. Neither platform
receives a host workspace mount; the VM's writable ext4 overlay provides
persistent workspace storage, matching the OpenShell VM model. The Gateway
request builder now expresses this storage mode, but actual Gateway VM creation
remains disabled.

The Gateway rejects VM create requests until the OpenShell-style host supervisor
and authenticated host/guest isolation contract are implemented, and Linux KVM
and macOS ARM64 Gateway lanes pass.

OCI image `Entrypoint`, `Cmd`, `Env`, `User` and `WorkingDir` are applied to the
in-memory VM workload spec. Gateway recovery reloads the image config by the
pinned platform manifest digest. The driver keeps image environment and user
values out of durable state, filters reserved identity variables, and rejects
root identities. Unit tests and macOS ARM64 live acceptance cover OCI config,
private-registry credential-helper auth, digest-pinned recovery, one-shot TTY exec and
image env/workdir inheritance. The same live run covers PTY resize, stdin and
Ctrl-C delivery. Gateway-to-VM and Linux KVM acceptance, host supervisor/isolation, GPU
passthrough and PID limits remain open. These gaps are tracked in the
[runtime acceptance matrix](../reference/runtime-acceptance.md).

## Host setup

Build the worker, static Linux guest agent, umoci and runtime helpers with
`quarshell-driver/scripts/build-vm-runtime.sh`. Set `CAUTEM_VM_WORKER`,
`CAUTEM_VM_GUEST_AGENT` and OpenShell's `OPENSHELL_VM_RUNTIME_DIR` for the
target host. VM state defaults to `$XDG_STATE_HOME/openshell/vm-driver` (or
`$HOME/.local/state/openshell/vm-driver`); set
`OPENSHELL_VM_DRIVER_STATE_DIR` to override it. Rootfs tar uploads stage under
`<state_dir>/rootfs-tar-staging` by default; set `rootfs_tar_staging_dir` or
`OPENSHELL_VM_ROOTFS_TAR_STAGING_DIR` to choose another directory.
When `OPENSHELL_VM_RUNTIME_DIR` is unset, the Go driver looks for the runtime
bundle under `$XDG_DATA_HOME/openshell/vm-runtime` (or
`$HOME/.local/share/openshell/vm-runtime`). The bundle still must be installed
by the operator; unlike upstream's embedded build resources, this Go build does
not yet embed or extract libkrun.
The build uses umoci v0.6.0 and verifies its SHA-256 against
`quarshell-driver/scripts/vm-runtime-artifacts.lock`. Add a verified digest to the
lock before selecting another version. A custom `CAUTEM_VM_UMOCI` binary
requires `CAUTEM_VM_UMOCI_SHA256`; the binary is verified before publication to
the runtime directory. A complete build writes `vm-runtime-manifest.json` with
hashes for all seven artifacts; verification rejects missing, changed and
unexpected files. Linux requires readable and writable `/dev/kvm`; macOS
support targets Apple Silicon with Hypervisor.framework. Gateway preflight
requires the macOS worker to be signed by a trusted Apple Developer team: set `CAUTEM_VM_CODESIGN_IDENTITY` when
building and the matching `CAUTEM_VM_WORKER_TEAM_ID` when starting Gateway. The
preflight runs `codesign --verify --strict` and compares the Team ID; an ad-hoc
signature is rejected. Rootfs preparation also requires e2fsprogs.

For two-disk Create, the host stages tar or OCI layout onto a temporary ext4
disk, launches the image-prep worker on the pinned bootstrap, and verifies
`/image-rootfs` before publishing an immutable workload cache entry. This passes
live macOS ARM64 acceptance; Linux KVM lanes and the complete failure/security
matrix remain release gates.

Set `CAUTEM_VM_IMAGE_CACHE_MAX_BYTES` to change the default 64 GiB image cache
quota. Registry credentials come from the Docker or Podman credential files
available to the driver process. Uploaded rootfs tar files are limited to 10
GiB compressed and 32 GiB expanded.

For live driver acceptance, follow the VM section in the
[driver README](https://github.com/KLORQ/quarshell-driver#microvm-status). Gateway
and Linux KVM acceptance remain release gates.


See the [MicroVM pre-release plan](microvm-release-plan.md) for the full parity
backlog, production phases, test matrix, and release criteria.
