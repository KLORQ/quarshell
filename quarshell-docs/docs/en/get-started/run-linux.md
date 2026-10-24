<!--
SPDX-FileCopyrightText: Copyright (c) 2026 the Quarshell authors
SPDX-License-Identifier: Apache-2.0
-->

# Run Quarshell on Linux

This guide runs the CLI and Gateway on the Linux host and creates a sandbox
with Docker Engine. Quarshell APIs are pre-stable and remain in the `0.x`
series.

## Requirements

- A 64-bit Linux system on `x86_64` or `arm64`.
- Docker Engine with a working API socket, or Podman with its API socket.
- `curl`, `sh`, and a shell with `~/.local/bin` on `PATH`.

The user running `quarshell-gateway` must be allowed to access the container
engine socket. Access to the rootful Docker socket grants root-equivalent
control over that host; use a dedicated machine and follow your host's security
policy.

## 1. Install the container engine

For Docker, follow the official guide for your distribution, for example
[Ubuntu](https://docs.docker.com/engine/install/ubuntu/). Start the service and
make sure the current user can connect to it. Then verify:

```sh
docker version
docker info
docker run --rm hello-world
```

For Podman, follow the [official installation instructions](https://podman.io/docs/installation).
On a systemd user session, start its socket:

```sh
systemctl --user enable --now podman.socket
podman info
```

## 2. Install the CLI and Gateway

```sh
curl -fsSL https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh | QUARSHELL_NO_START=1 sh
```

The installer verifies the SHA-256 checksum and installs `quarshell`,
`quarshell-gateway`, and helper assets into `~/.local/bin`. `QUARSHELL_NO_START=1`
lets you select and start the engine before launching Gateway.

If needed, add the install directory to `PATH` in `~/.profile`:

```sh
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> ~/.profile
. ~/.profile
```

Verify the CLI and Gateway:

```sh
quarshell version
```

## 3. Select Docker or Podman

The local Gateway defaults to Docker. For Podman, select it before starting the
Gateway so the Gateway process inherits the backend setting:

```sh
export QUARSHELL_GATEWAY_RUNTIME=podman
```

Keep this variable in the shell used to run `quarshell`. If a healthy local
Gateway was started with a different backend, restart it after changing the
setting. Confirm engine access as the same user that runs the Gateway:

```sh
docker info   # Docker
podman info   # Podman
quarshell gateway ensure
quarshell status
```

## 4. Pull the image and prepare a policy

For Docker:

```sh
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
```

For Podman, run `podman pull ghcr.io/klorq/quarshell/sandboxes/base:latest`.
Resolve the `base` alias to
the published image and create a policy file:

```sh
export QUARSHELL_USE_GHCR=1
mkdir -p ~/.config/quarshell
cat > ~/.config/quarshell/default-policy.yaml <<'EOF'
version: 1
filesystem_policy:
  include_workdir: true
  read_only:
    - /usr
    - /lib
    - /proc
    - /dev/urandom
    - /app
    - /etc
    - /var/log
    - /AGENTS.md
  read_write:
    - /tmp
    - /dev/null
    - /sandbox
    - /workspace
landlock:
  compatibility: best_effort
network_policies: {}
EOF
```

## 5. Create and use a sandbox

Run from the project directory you want to upload:

```sh
quarshell sandbox create \
  --name demo \
  --gateway-workspace default \
  --from base \
  --policy ~/.config/quarshell/default-policy.yaml \
  --upload "$PWD"

quarshell sandbox list
quarshell sandbox exec demo -- uname -a
quarshell sandbox connect demo
```

`connect` opens a shell; leave it with `exit`. `--upload` sends a copy of the
directory during creation. It does not mount the live host directory. The
policy allows no network destinations until you add explicit rules; see
[network policy](../guides/policy.md).

Delete the sandbox when finished:

```sh
quarshell sandbox delete demo
```

## Troubleshooting

- Gateway health check fails: run `quarshell gateway ensure` and inspect
  `~/.config/quarshell/gateway.log`.
- `docker info` or `podman info` fails: fix engine service/socket access for
  the same user, then retry.
- Image pull fails: verify registry access with the engine's `pull` command.
- Podman is ignored: export `QUARSHELL_GATEWAY_RUNTIME=podman` before starting the
  Gateway. This guide installs with `QUARSHELL_NO_START=1`, so the installer does
  not start the default Docker backend first.
