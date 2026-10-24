<!--
SPDX-FileCopyrightText: Copyright (c) 2026 the Quarshell authors
SPDX-License-Identifier: Apache-2.0
-->

# Run Quarshell on macOS

This guide runs the CLI and Gateway on your Mac and starts the workload in a
Docker container hosted by Docker Desktop. Quarshell APIs are pre-stable and
remain in the `0.x` series.

## Requirements

- macOS on Intel (`x86_64`) or Apple silicon (`arm64`). The installer selects
  the matching release archive.
- Docker Desktop, started and signed in if your organization requires it.
  Install it using the [official macOS guide](https://docs.docker.com/desktop/setup/install/mac-install/).
- `curl`, `sh`, and a shell with `~/.local/bin` on `PATH`.

Docker Desktop runs the container engine in a Linux VM. Quarshell's Gateway
and CLI run on macOS; the sandbox itself runs in that engine VM. Use `--upload`
to send a project directory into the sandbox.

## 1. Install the CLI and Gateway

```sh
curl -fsSL https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh | QUARSHELL_NO_START=1 sh
```

The installer verifies the SHA-256 checksum, then installs `quarshell`,
`quarshell-gateway`, and helper assets into `~/.local/bin`. `QUARSHELL_NO_START=1`
lets you start the engine first and then launch Gateway with the selected backend.

If the installer reports that `~/.local/bin` is not on `PATH`, add this to
`~/.zshrc`:

```sh
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> ~/.zshrc
source ~/.zshrc
```

Confirm the installation:

```sh
quarshell version
```

## 2. Start Docker Desktop

Open Docker Desktop and wait until it reports that the engine is running. In a
Terminal window, verify that the CLI can reach it:

```sh
docker version
docker info
quarshell doctor
quarshell gateway ensure
quarshell status
```

The Docker client must work without `sudo`. Keep Docker Desktop running while
you use Quarshell.

## 3. Pull the sandbox image

```sh
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
export QUARSHELL_USE_GHCR=1
```

The environment variable makes the CLI's `base` alias use this published
image. Add the `export` line to `~/.zshrc` to keep it in new Terminal windows.

## 4. Create and use a sandbox

From the project directory you want to work on, create a local policy file:

```sh
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

Create a sandbox, upload the current directory, then run a command inside it:

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

`connect` opens a shell; leave it with `exit`. The upload is a copy made during
creation, not a live host-directory mount. The policy starts with no allowed
network destinations. Add explicit rules before running an agent that needs
network access; see [network policy](../guides/policy.md).

Remove the sandbox when finished:

```sh
quarshell sandbox delete demo
```

## Podman alternative

Install Podman from the [official instructions](https://podman.io/docs/installation),
then initialize and start its Linux VM:

```sh
podman machine init
podman machine start
podman info
```

Choose Podman before starting the local Gateway, because the Gateway owns the
compute backend:

```sh
export QUARSHELL_GATEWAY_RUNTIME=podman
quarshell gateway ensure
```

Use `podman pull ghcr.io/klorq/quarshell/sandboxes/base:latest` for the image,
then keep `QUARSHELL_USE_GHCR=1` set in this shell.

## Troubleshooting

- `quarshell status` cannot reach the Gateway: run `quarshell gateway ensure`, then
  inspect `~/Library/Application Support/quarshell/gateway.log`.
- `docker info` fails: start Docker Desktop and check that its current context
  is selected with `docker context ls`.
- Image pull fails: verify registry access with `docker pull` and try again.
- `quarshell doctor` reports a missing Docker prerequisite: make sure `docker`
  works in the same Terminal session as `quarshell`.
