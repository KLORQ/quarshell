<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Docker provider

A **compute provider** is the engine that runs the sandbox and its egress
proxy. It is picked with `QUARSHELL_DRIVER` and shown by `quarshell status`.

| Provider | `QUARSHELL_DRIVER` | Status |
|----------|---------------------|--------|
| Docker Engine 28.0+ on Linux | `docker` | <span class="ws-badge ws-badge--ok">supported, default</span> |
| Docker Desktop on macOS/Windows | `docker` | <span class="ws-badge ws-badge--ok">beta</span> |
| Podman 6+ on Linux, rootful/rootless | `podman` | minimum for proxy isolation; qualification ongoing |
| Podman 5.x on Linux | `podman` | lifecycle subset; proxy host-gateway isolation is unavailable |
| [Kubernetes](../kubernetes.md) | `kubernetes` | <span class="ws-badge ws-badge--soon">coming soon</span> |
| [MicroVM](../microvm.md) | `vm` | <span class="ws-badge ws-badge--soon">coming soon</span> |

Docker is the default. quarshell talks to the local Engine API
(`DOCKER_HOST` / Desktop socket), creates an internal network per sandbox,
starts an egress proxy sidecar, then starts the sandbox container.

The minimum Docker Engine baseline is 28.0. [OpenShell's installation guide]
documents Docker 28.0+ and Podman 5.x. Our Docker gateway E2E passes on 28.0;
the Driver container suite passes on 28.0.4. OpenShell accepts Podman 5.x for
the general contract; our required proxy/Gateway lane now uses Podman 6.1.4
because host-gateway isolation is verified with a blackhole route. Rootful and
rootless Podman 6.1.4 qualification is configured in CI; a new passing run is
still required after this runner change.
Docker Desktop requires a direct smoke on each supported host release. WSL2
gateway reachability is documented below but is not yet a release-gated
platform. Kubernetes and MicroVM are not production providers in this release.

[OpenShell's installation guide]: https://github.com/NVIDIA/OpenShell/blob/main/docs/about/installation.mdx

## Layout (one sandbox)

```text
host Docker Engine
├── quarshell-<name>              sandbox (agent image)
├── quarshell-proxy-<name>        egress sidecar (slim base)
├── quarshell-net-<name>          internal network
├── quarshell-data-<name>         optional data volume
└── quarshell-ca-<name>           MITM CA volume (when proxy enabled)
```

## Activate

```bash
unset QUARSHELL_DRIVER          # or: export QUARSHELL_DRIVER=docker
quarshell doctor
quarshell status
```

Expect `driver: docker` in status output.

## Private registries

After `docker login registry.example`, image pulls use the Docker CLI
credential config for the user running the gateway (including configured
credential helpers). The credentials are sent only with the Engine image-pull
request; they are not copied into sandbox configuration or environment.

## Private-CA HTTPS endpoints

Set `egress_ca_bundle` in the Docker compute-driver configuration to a PEM
bundle containing operator-managed roots for inspected HTTPS destinations. It
extends system trust and keeps certificate and hostname verification enabled.
This is separate from `proxy_ca_bundle`, which trusts an HTTPS corporate
forward proxy. Recreate the sandbox proxy after rotating the bundle.

For rootless engines whose UID/GID mapping changes, `reconcile_data_ownership`
can be enabled in the Docker compute-driver configuration. When enabled for a
sandbox with persistent data, the privileged init process reassigns entries in
the dedicated `/quarshell/data` volume to the sandbox user at startup. This is
off by default, can take time on large volumes, and does not touch the workspace
bind mount.

### Docker Desktop on WSL2

The gateway rewrites its loopback listener to `host.docker.internal` for the
sandbox supervisor. If a WSL2 sandbox still cannot reach the gateway, set the
Docker compute driver's `grpc_endpoint` explicitly, for example
`https://host.docker.internal:17670`. For HTTPS, configure `guest_tls_ca`,
`guest_tls_cert`, and `guest_tls_key` together. The endpoint must be reachable
from Docker Desktop's container network.

!!! tip "Not the same as credential providers"
    `--provider github`, `--provider cursor` and friends attach secrets and
    egress rules to a sandbox. They work with any compute provider — see
    [Credential providers](../../guides/credentials.md).
