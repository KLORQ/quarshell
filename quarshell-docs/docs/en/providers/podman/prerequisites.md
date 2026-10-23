<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Podman prerequisites

## Linux (rootless)

```bash
systemctl --user enable --now podman.socket
export QUARSHELL_DRIVER=podman
# export QUARSHELL_PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock
# or: export DOCKER_HOST=unix:///run/user/$UID/podman/podman.sock

podman info
quarshell status
```

## macOS (Podman Machine)

```bash
podman machine init     # once
podman machine start
export QUARSHELL_DRIVER=podman
quarshell status
```

Discovery also checks
`~/.local/share/containers/podman/machine/podman.sock` and `podman info`.

## Images

Use the same sandbox tags as Docker (`quarshell-sandbox:local`, `:cursor`, …
or GHCR catalog). Build with Docker tooling on a host that can push/load into
Podman’s store, or `podman pull` / `podman load` equivalents.

```bash
podman images | grep quarshell
```

## Gateway

Gateway still runs on the host (binary or compose). Select it before create with
providers or proxy enabled — same as Docker.

## AppArmor profile

The Podman backend accepts `app_armor_profile` in its gateway driver table:

```toml
[openshell.drivers.podman]
app_armor_profile = "RuntimeDefault"
```

Supported values are `RuntimeDefault`, `Unconfined`, and
`Localhost/<profile-name>`. `RuntimeDefault` and `Localhost/...` require
Podman's system info to report AppArmor support; configuration fails before sandbox
creation otherwise. `Unconfined` explicitly disables AppArmor confinement.

## Caller supplied driver configuration

Caller `template.driver_config` is disabled by default. Enable it in the Podman
backend table only when a workload needs Podman specific mounts or CDI devices:

```toml
[openshell.drivers.podman]
allow_driver_config = true
```

Named volume mounts must refer to an existing volume with the default admission
labels for the target workspace:

```bash
podman volume create \
  --label openshell.ai/sandbox-attachable=true \
  --label openshell.ai/sandbox-attachable-workspace=team-a \
  agent-cache
```

The backend checks these labels and the volume identity again before restart.
Bind and image mounts remain rejected while `resource_admission.enabled` is
true because Podman cannot provide a trusted label resolver for those sources.
Bind mounts also require `enable_bind_mounts = true`.
