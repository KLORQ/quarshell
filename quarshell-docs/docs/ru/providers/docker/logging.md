<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Логирование (Docker)

## Логи контейнеров

Sandbox и proxy используют Docker `json-file` с ротацией:

| Параметр | По умолчанию |
|----------|--------------|
| Driver | `json-file` |
| `max-size` | `10m` |
| `max-file` | `3` |

```bash
export QUARSHELL_DOCKER_LOG_DRIVER=json-file   # или none
export QUARSHELL_DOCKER_LOG_MAX_SIZE=10m
export QUARSHELL_DOCKER_LOG_MAX_FILE=3
```

`none` отключает Engine-логи, если достаточно observation в gateway.

Запасной вариант на весь демон:

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
```

## Observation ring

Gateway держит in-memory кольцо на sandbox (4096 строк, 1 MiB). При удалении
sandbox, которым управляет CLI, он сохраняет последние строки в каталоге
данных gateway:
до 1 MiB для каждого из последних 64 удалённых sandbox, не дольше 7 дней.
Архив остаётся доступен только своему workspace и читается без `--tail`;
подписаться на него нельзя. Новый sandbox с тем же именем очищает старый
архив. Для live-потока используйте:

```bash
quarshell logs demo --tail --source proxy
```
