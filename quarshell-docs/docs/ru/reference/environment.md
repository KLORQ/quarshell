<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Окружение

| Переменная | Назначение |
|------------|------------|
| `CAUTEM_DRIVER` | `docker` (default) \| `podman` |
| `CAUTEM_PODMAN_SOCKET` | Явный API-сокет Podman |
| `DOCKER_HOST` | Endpoint Engine API |
| `CAUTEM_DEFAULT_MEMORY` | Soft default create без `--memory` |
| `CAUTEM_DEFAULT_CPU` | Soft default без `--cpu` |
| `CAUTEM_SANDBOX_PIDS_LIMIT` | Лимит PIDs драйвера Gateway (`0` = без лимита) |
| `CAUTEM_DOCKER_LOG_DRIVER` | `json-file` \| `none` |
| `CAUTEM_DOCKER_LOG_MAX_SIZE` | json-file `max-size` (default `10m`) |
| `CAUTEM_DOCKER_LOG_MAX_FILE` | json-file `max-file` (default `3`) |
| `CAUTEM_SECRETS_KEK` | Ключ secrets gateway |
| `OPENSHELL_LOG_LEVEL` | Уровень логов Gateway daemon |

Конфиг: `~/.config/cautem/config.yaml` (`defaults:`, `images:`, gateways).
