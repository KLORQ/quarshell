<!--
SPDX-FileCopyrightText: Copyright (c) 2026 the Quarshell authors
SPDX-License-Identifier: Apache-2.0
-->

# Run a local VM sandbox

This quickstart builds the Go CLI, Gateway, and Linux guest runtime from the
workspace, then starts a sandbox in a hardware-virtualized Linux VM. VM support
is experimental; the public API remains `0.x`.

## Requirements

- Go and Task versions required by the checked-out workspace, Git, Python 3,
  and `curl`.
- **macOS:** Apple silicon (arm64), Hypervisor.framework, Homebrew, and
  `libkrun`, `libkrunfw`, and `e2fsprogs` (`brew install libkrun libkrunfw e2fsprogs`).
- **Linux:** amd64 or arm64, KVM enabled with read/write access to `/dev/kvm`,
  a compatible `libkrun` runtime bundle, and `e2fsprogs` (`mke2fs`). Run the
  Gateway as an unprivileged user; do not run it with `sudo`.
- Network access to download the pinned `umoci` build dependency and the
  selected OCI image.

## Build and configure

Check out the workspace and its pinned standalone modules, then build the
runtime helpers and host binaries:

```sh
git clone https://github.com/KLORQ/quarshell-workspace.git
cd quarshell-workspace
tools/checkout-modules.sh
task workspace:sync

export VM_ASSETS="$PWD/quarshell-driver/dist/vm-local"
mkdir -p "$VM_ASSETS/libkrun"
QUARSHELL_VM_BUILD_DIR="$VM_ASSETS" quarshell-driver/scripts/build-vm-runtime.sh
go build -C quarshell-cli -o "$VM_ASSETS/quarshell" ./cmd/quarshell
go build -C quarshell-gateway -o "$VM_ASSETS/quarshell-gateway" ./cmd/quarshell-gateway
export PATH="$VM_ASSETS:$PATH"
```

On macOS, link the Homebrew hypervisor libraries and pin the locally built,
ad-hoc-signed worker. This setup targets Apple silicon:

```sh
ln -sf "$(brew --prefix libkrun)/lib/libkrun.dylib" "$VM_ASSETS/libkrun/libkrun.dylib"
ln -sf "$(brew --prefix libkrunfw)/lib/libkrunfw.5.dylib" "$VM_ASSETS/libkrun/libkrunfw.5.dylib"
export QUARSHELL_VM_WORKER_SHA256="$(shasum -a 256 "$VM_ASSETS/quarshell-vm-worker" | awk '{print $1}')"
export OPENSHELL_VM_RUNTIME_DIR="$VM_ASSETS/libkrun"
```

On Linux, set `OPENSHELL_VM_RUNTIME_DIR` to an operator-provided bundle
containing `libkrun.so` and `libkrunfw.so.5`:

```sh
export OPENSHELL_VM_RUNTIME_DIR=/path/to/libkrun-bundle
```

Keep this value when setting the Gateway environment below.

Point the local Gateway at the built runtime. `QUARSHELL_GATEWAY_SERVICE=0` keeps
it in this shell's environment instead of starting it through the OS service
manager. Set these variables before the first `gateway ensure`; an already
running Gateway must be stopped and restarted after changing its backend.

```sh
export QUARSHELL_GATEWAY_RUNTIME=vm
export QUARSHELL_GATEWAY_SERVICE=0
export QUARSHELL_VM_WORKER="$VM_ASSETS/quarshell-vm-worker"
export QUARSHELL_VM_GUEST_AGENT="$VM_ASSETS/quarshell-vm-guest-agent-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_INIT_BINARY="$VM_ASSETS/quarshell-init-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_SUPERVISOR_BINARY="$VM_ASSETS/quarshell-driver-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_SSHD_BINARY="$VM_ASSETS/quarshell-sshd-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_PROXY_BINARY="$VM_ASSETS/quarshell"
export QUARSHELL_RUNTIME_POLICY_PATH="$PWD/quarshell-cli/policies/default.yaml"
```

On Linux, `go env GOARCH` must match the host architecture (`amd64` or
`arm64`). On macOS, it selects the Linux guest architecture (`arm64`). The
Gateway creates its local TLS credentials when it starts.

## Start a sandbox

Start the VM-backed Gateway and select the published base image:

```sh
quarshell gateway ensure
quarshell status
export QUARSHELL_USE_GHCR=1
```

From the project directory you want to copy into the guest:

```sh
quarshell sandbox create --name vm-demo --from base --upload "$PWD"
quarshell sandbox exec vm-demo -- uname -a
quarshell sandbox connect vm-demo
quarshell sandbox delete vm-demo
```

`--upload` copies the project into the guest; it does not mount the host
directory. The VM sandbox runs a Linux workload on both host platforms. For the
module map and each repository's role, see [Project modules](../concepts/modules.md).

## Troubleshooting

- **Worker trust error on macOS:** rebuild the worker, recalculate
  `QUARSHELL_VM_WORKER_SHA256`, and restart Gateway. Use a Team ID for distributed
  signed builds.
- **KVM preflight fails on Linux:** check `test -r /dev/kvm && test -w /dev/kvm`
  as the same unprivileged user that runs Gateway.
- **Runtime library error:** check that the configured bundle contains the two
  platform-specific `libkrun` files listed above.
- **Gateway still reports Docker:** stop the old local Gateway, export the VM
  settings in the same shell, then run `quarshell gateway ensure` again.
