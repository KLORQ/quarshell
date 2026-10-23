<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Окружение

| Переменная | Назначение |
|------------|------------|
| `QUARSHELL_DRIVER` | `docker` (default) \| `podman` |
| `QUARSHELL_PODMAN_SOCKET` | Явный API-сокет Podman |
| `DOCKER_HOST` | Endpoint Engine API |
| `QUARSHELL_DEFAULT_MEMORY` | Soft default create без `--memory` |
| `QUARSHELL_DEFAULT_CPU` | Soft default без `--cpu` |
| `QUARSHELL_SANDBOX_PIDS_LIMIT` | Лимит PIDs драйвера Gateway (`0` = без лимита) |
| `QUARSHELL_DOCKER_LOG_DRIVER` | `json-file` \| `none` |
| `QUARSHELL_DOCKER_LOG_MAX_SIZE` | json-file `max-size` (default `10m`) |
| `QUARSHELL_DOCKER_LOG_MAX_FILE` | json-file `max-file` (default `3`) |
| `QUARSHELL_SECRETS_KEK` | Ключ secrets gateway |
| `OPENSHELL_LOG_LEVEL` | Уровень логов Gateway daemon |

Конфиг: `~/.config/quarshell/config.yaml` (`defaults:`, `images:`, gateways).
