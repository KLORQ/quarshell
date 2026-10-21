<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Устранение неполадок (Docker)

| Симптом | Действие |
|---------|----------|
| `docker not available` | Запустите Engine/Desktop; проверьте `docker info` и `DOCKER_HOST` |
| RSS VM Desktop > 10+ GiB | Docker VMM; `--memory`; ротация логов; prune остановленных контейнеров |
| Нет образа | `task runtime:image:cli` или pull GHCR |
| Proxy не достучится до gateway | Выберите gateway; sidecar → `host.cautem.internal` |
| `CONNECT 403` / policy deny | Узкий allow через `cautem rule approve` / policy set |
| В guest сырые секреты | `--provider`, не `--env KEY=secret` |
| Cursor `invalid API key` | Не инжектить `CURSOR_API_KEY`; `agent login` в guest |
| Landlock spam на Desktop | Нормально при ABI 0; граница — контейнер |
| Старые лимиты после апгрейда | Пересоздайте sandbox — LogConfig/PidsLimit на create |

```bash
cautem doctor
docker ps -a --filter label=cautem.sandbox=1
docker stats --no-stream
cautem logs <name> --source proxy
```
