---
title: Remaining OpenShell migration work
---

# Remaining OpenShell migration work

Quarshell targets pinned OpenShell `v0.1.4`. Compatibility remains partial until
behavioral acceptance covers every required RPC, provider, runtime, client and
backend feature. Matching schemas and registered handlers are not sufficient.

The current hosted root CI, Linux KVM lifecycle, nested Docker and rootful/rootless
Podman acceptance pass. Local macOS/HVF lifecycle and Gateway composition pass.
These results close the old KVM-environment gate, not every VM parity row.

Remaining work is grouped into these tracks:

- Gateway authentication, token/OAuth lifecycle, incomplete peer RPCs and
  functional acceptance for every public RPC.
- Multi-Gateway ownership/reconnect behavior and the at-least-once stream cursor
  boundary.
- Linux seccomp notification, descendant lifecycle and raw-socket/DNS/Unix-socket
  bypass checks.
- Kubernetes real-cluster acceptance and full Docker/Podman config/network
  parity.
- Remaining VM feature rows, Windows MXC runtime acceptance, and GPU/VFIO
  hardware support.
- CLI/TUI, SDK, Python/TypeScript client and symbolic prover behavior.
- Published-consumer install, upgrade/rollback, images, and signed macOS release.

See the [current engineering plan](https://github.com/KLORQ/quarshell-workspace/blob/main/plans/REMAINING.md),
the [OpenShell compatibility table](openshell-compatibility.md), and the
[runtime acceptance matrix](runtime-acceptance.md) for feature owners and
acceptance evidence. MXC and GPU/VFIO remain blocked until compatible hardware
and runtime dependencies are available.
