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

The latest published CLI is `v0.1.6`. Pin it when installing so this
guide keeps working until the next stable release is published.
Open a terminal and run:

```bash
curl -LsSf https://raw.githubusercontent.com/cautem/quarshell-cli/main/install.sh \
  | CAUTEM_VERSION=v0.1.6 sh
cautem version
```

The binary goes to `~/.local/bin` — make sure it is on your `PATH`.

To install into another directory:

```bash
curl -LsSf https://raw.githubusercontent.com/cautem/quarshell-cli/main/install.sh \
  | CAUTEM_VERSION=v0.1.6 CAUTEM_INSTALL_DIR=/usr/local/bin sh
```

### from source

Build from the workspace checkout if you want the latest `main` or work on
cautem itself:

```bash
cd /path/to/cautem
export GOWORK=$PWD/go.work

go build -C quarshell-cli -o cautem ./cmd/cautem
./quarshell-cli/cautem install
./quarshell-cli/cautem version
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
