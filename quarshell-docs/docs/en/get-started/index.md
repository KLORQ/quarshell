<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Get started

quarshell is a single CLI that runs coding agents in policy-bound sandboxes on
Docker or Podman. Install the CLI, make sure a container engine is running and
pull a sandbox image — then [start the gateway](gateway.md) and
[create your first sandbox](first-sandbox.md).

## Install the CLI

For a complete platform-specific walkthrough, see [macOS](run-macos.md) or
[Linux](run-linux.md).

### with the installer <small>recommended</small> { #with-installer data-toc-label="with the installer" }

The installer downloads a verified release archive from `quarshell-cli`, installs
the CLI and its helper assets into `~/.local/bin`, and starts the local Gateway
when that platform release includes it. Windows releases currently include the
CLI only. Run:

```bash
curl -LsSf https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh | sh
quarshell version
quarshell status
```

`quarshell status` should report `Connected`. Set `QUARSHELL_NO_START=1` to skip
gateway startup, or `QUARSHELL_VERSION=vX.Y.Z` to pin a release.

If you use Podman, initialize it first and install with `QUARSHELL_NO_START=1`.
Then export `QUARSHELL_GATEWAY_RUNTIME=podman` before running
`quarshell gateway ensure`; otherwise the installer starts the default Docker backend.

The binaries and helper assets go to `~/.local/bin` — make sure it is on your `PATH`.

To install into another directory:

```bash
curl -LsSf https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh \
  | QUARSHELL_INSTALL_DIR=/usr/local/bin sh
```

### from source

Build from the workspace checkout if you want the latest published module
versions or want to work across modules:

```bash
git clone https://github.com/KLORQ/quarshell-workspace.git
cd quarshell-workspace
tools/checkout-modules.sh
task workspace:sync

go build -C quarshell-cli -o quarshell ./cmd/quarshell
go build -C quarshell-gateway -o quarshell-gateway ./cmd/quarshell-gateway
./quarshell install
./quarshell version
```

## Start a container engine

quarshell needs a running compute provider. Docker is the default; Podman
works through the same Engine API.

=== "Docker"

    Docker Engine 28.0+ or Docker Desktop. On macOS and Windows prefer the
    **Docker VMM** backend.

    ```bash
    docker info
    quarshell doctor
    quarshell status
    ```

=== "Podman on macOS"

    ```bash
    podman machine init     # once
    podman machine start
    export QUARSHELL_GATEWAY_RUNTIME=podman
    quarshell gateway ensure
    quarshell status
    ```

=== "Podman on Linux"

    ```bash
    systemctl --user enable --now podman.socket
    export QUARSHELL_GATEWAY_RUNTIME=podman
    quarshell gateway ensure
    quarshell status
    ```

`quarshell status` should report that the selected Gateway is connected.
Engine details: [Docker](../providers/docker/index.md) ·
[Podman](../providers/podman/index.md).

## Get a sandbox image

Pull the base image — it has the CLI tooling an agent needs:

```bash
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
export QUARSHELL_USE_GHCR=1
```

!!! tip "Building images yourself"
    From the workspace root: `task runtime:image:cli` builds the base image and
    `task docker:agent:cursor` the Cursor agent. The full list is in
    [Images](../reference/images.md).
