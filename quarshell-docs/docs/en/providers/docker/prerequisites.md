<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Docker prerequisites

## Engine

- Docker Engine 28.0+ or Docker Desktop with a working API socket.
- Linux: `/var/run/docker.sock` or rootless user socket.
- macOS / Windows: Docker Desktop; prefer the current **Docker VMM** backend for
  better memory reclaim under load.
- Podman 5.x+ on Linux runs the lifecycle subset. Podman 6+ is required for the
  current full proxy-backed sandbox path because older releases lack the
  required host-gateway blocking route. Rootful and rootless modes have
  different tested coverage; see the provider support table.

```bash
docker version
docker info
```

## Images

Pull or build at least one sandbox image before create:

```bash
# Local workspace build
task runtime:image:cli          # quarshell-sandbox:local
task docker:agent:cursor        # quarshell-sandbox:cursor

# Or GHCR catalog
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
```

Catalog and BYOC: [Images reference](../../reference/images.md).

## Gateway (recommended)

Proxy, secrets, and provider attach require a reachable gateway:

```bash
quarshell gateway ensure
quarshell gateway add https://127.0.0.1:17670 --local --name local
quarshell gateway select local
```

Gateway Compose file: `quarshell-gateway/compose/docker-compose.yml`.

## AppArmor profile

The Docker backend accepts `app_armor_profile` in its gateway driver table:

```toml
[openshell.drivers.docker]
app_armor_profile = "RuntimeDefault"
```

Supported values are `RuntimeDefault`, `Unconfined`, and
`Localhost/<profile-name>`. `RuntimeDefault` and `Localhost/...` require the
Engine host to report AppArmor support; sandbox creation fails before creating
resources otherwise. `Unconfined` explicitly disables AppArmor confinement.

## Caller supplied driver configuration

Caller `template.driver_config` is disabled by default. Enable it for the Docker
backend only when the workload needs Docker specific mounts or CDI devices:

```toml
[openshell.drivers.docker]
allow_driver_config = true
```

Named volume mounts must refer to an existing volume labeled for attachment to
the sandbox workspace:

```bash
docker volume create \
  --label openshell.ai/sandbox-attachable=true \
  --label openshell.ai/sandbox-attachable-workspace=team-a \
  agent-cache
```

The default `resource_admission` policy requires both labels and the workspace
identity to match. Quarshell records the admitted volume identity and checks it
again before restart. Bind and image mounts cannot be verified through engine
labels, so they remain rejected while admission is enabled; bind mounts also
require `enable_bind_mounts = true`.

## Desktop memory

Uncapped agent containers and unbounded container logs inflate the Desktop VM.
Set soft defaults (optional) and keep log rotation enabled (default):

```yaml
# ~/.config/quarshell/config.yaml
defaults:
  memory: 2g
  cpu: 2
```

See [Resources and limits](./resources.md) and [Logging](./logging.md).
