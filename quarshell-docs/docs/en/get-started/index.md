<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Get started

cautem is a single CLI that runs coding agents in policy-bound sandboxes on
Docker or Podman. Install the CLI, make sure a container engine is running and
pull a sandbox image — then [start the gateway](gateway.md) and
[create your first sandbox](first-sandbox.md).

## Install the CLI

### with the installer <small>recommended</small> { #with-installer data-toc-label="with the installer" }

The installer downloads the CLI and gateway, installs them into
`~/.local/bin`, starts a local gateway and registers it. Run:

```bash
curl -LsSf https://raw.githubusercontent.com/KLORQ/quarshell/master/quarshell-cli/install.sh | sh
cautem version
cautem status
```

`cautem status` should report `Connected`. Set `CAUTEM_NO_START=1` to skip
gateway startup, or `CAUTEM_VERSION=vX.Y.Z` to pin a release.

The binary goes to `~/.local/bin` — make sure it is on your `PATH`.

To install into another directory:

```bash
curl -LsSf https://raw.githubusercontent.com/KLORQ/quarshell/master/quarshell-cli/install.sh \
  | CAUTEM_INSTALL_DIR=/usr/local/bin sh
```

### from source

Build from the workspace checkout if you want the latest `main` or work on
cautem itself:

```bash
cd /path/to/cautem
export GOWORK=$PWD/go.work

go build -C quarshell-cli -o cautem ./cmd/cautem
go build -C quarshell-gateway -o quarshell-gateway ./cmd/quarshell-gateway
./cautem install
./cautem version
```

## Start a container engine

cautem needs a running compute provider. Docker is the default; Podman
works through the same Engine API.

=== "Docker"

    Docker Engine 28.0+ or Docker Desktop. On macOS and Windows prefer the
    **Docker VMM** backend.

    ```bash
    docker info
    cautem doctor
    cautem status
    ```

=== "Podman on macOS"

    ```bash
    podman machine init     # once
    podman machine start
    export CAUTEM_DRIVER=podman
    cautem status
    ```

=== "Podman on Linux"

    ```bash
    systemctl --user enable --now podman.socket
    export CAUTEM_DRIVER=podman
    cautem status
    ```

`cautem status` should print the driver you picked (`docker` or `podman`).
Engine details: [Docker](../providers/docker/index.md) ·
[Podman](../providers/podman/index.md).

## Get a sandbox image

Pull the base image — it has the CLI tooling an agent needs:

```bash
docker pull ghcr.io/cautem/cautem/sandboxes/base:latest
```

!!! tip "Building images yourself"
    From the workspace root: `task runtime:image:cli` builds the base image and
    `task docker:agent:cursor` the Cursor agent. The full list is in
    [Images](../reference/images.md).
