<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Podman provider

Podman is a first-class compute provider. The Gateway registers Podman as a
native modular backend and uses the Docker-compatible Engine API client for
container operations. Sandbox labels, networks, proxy sidecar, and exec flows
match Docker.

## Activate

```bash
export QUARSHELL_DRIVER=podman
# optional:
# export QUARSHELL_PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock

quarshell status    # driver: podman
```

## Private registries

Run `podman login registry.example` before creating a sandbox. Pulls use the
matching entry from `REGISTRY_AUTH_FILE`, `$XDG_RUNTIME_DIR/containers/auth.json`,
or `~/.config/containers/auth.json`, in that order. Set the Podman compute
driver's `registry_auth_file` to use another authfile. If no matching Podman
entry exists, quarshell falls back to the Docker CLI credential config. Credentials
are sent only with the image-pull request and are not copied into the sandbox.

For inspected HTTPS endpoints signed by a private CA, set `egress_ca_bundle`
in the Podman compute-driver configuration to a PEM bundle. It extends system
trust while preserving certificate and hostname verification. `proxy_ca_bundle`
is separate and trusts the HTTPS forward proxy itself. Recreate the sandbox
proxy after CA rotation.

For rootless Podman installations where UID/GID mappings can change,
`reconcile_data_ownership` may be enabled in the compute-driver configuration.
For sandboxes with persistent data, startup then reassigns entries in the
dedicated `/quarshell/data` volume to the sandbox user. It is off by default,
can take time on large volumes, and does not touch the workspace bind mount.

Resource flags, log rotation, slim proxy image, and soft defaults behave the
same as on Docker — see [Docker resources](../docker/resources.md) and
[Docker logging](../docker/logging.md).
