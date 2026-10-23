<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Environment

| Variable | Purpose |
|----------|---------|
| `QUARSHELL_DRIVER` | `docker` (default) \| `podman` |
| `QUARSHELL_PODMAN_SOCKET` | Explicit Podman API socket |
| `DOCKER_HOST` | Engine API endpoint |
| `QUARSHELL_DEFAULT_MEMORY` | Soft create default when `--memory` unset |
| `QUARSHELL_DEFAULT_CPU` | Soft create default when `--cpu` unset |
| `QUARSHELL_SANDBOX_PIDS_LIMIT` | Gateway driver PIDs default (`0` = unlimited) |
| `QUARSHELL_DOCKER_LOG_DRIVER` | `json-file` \| `none` |
| `QUARSHELL_DOCKER_LOG_MAX_SIZE` | json-file `max-size` (default `10m`) |
| `QUARSHELL_DOCKER_LOG_MAX_FILE` | json-file `max-file` (default `3`) |
| `QUARSHELL_SECRETS_KEK` | Gateway secrets key |
| `OPENSHELL_LOG_LEVEL` | Gateway daemon log level |

Config file: `~/.config/quarshell/config.yaml` (`defaults:`, `images:`, gateways).
