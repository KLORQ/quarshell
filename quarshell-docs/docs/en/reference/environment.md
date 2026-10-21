<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Environment

| Variable | Purpose |
|----------|---------|
| `CAUTEM_DRIVER` | `docker` (default) \| `podman` |
| `CAUTEM_PODMAN_SOCKET` | Explicit Podman API socket |
| `DOCKER_HOST` | Engine API endpoint |
| `CAUTEM_DEFAULT_MEMORY` | Soft create default when `--memory` unset |
| `CAUTEM_DEFAULT_CPU` | Soft create default when `--cpu` unset |
| `CAUTEM_SANDBOX_PIDS_LIMIT` | Gateway driver PIDs default (`0` = unlimited) |
| `CAUTEM_DOCKER_LOG_DRIVER` | `json-file` \| `none` |
| `CAUTEM_DOCKER_LOG_MAX_SIZE` | json-file `max-size` (default `10m`) |
| `CAUTEM_DOCKER_LOG_MAX_FILE` | json-file `max-file` (default `3`) |
| `CAUTEM_SECRETS_KEK` | Gateway secrets key |
| `OPENSHELL_LOG_LEVEL` | Gateway daemon log level |

Config file: `~/.config/cautem/config.yaml` (`defaults:`, `images:`, gateways).
