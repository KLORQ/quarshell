<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Ресурсы и лимиты

Запросы ресурсов передаются выбранному Gateway через нативный
OpenShell `CreateSandbox`. Docker driver и его конфигурация находятся на хосте
Gateway; CLI не меняет локальные Docker-контейнеры.

| Параметр CLI | Поле OpenShell |
|--------------|----------------|
| `--cpu` | Запрос CPU |
| `--memory` | Запрос памяти |
| `--gpu [COUNT]` | Запрос GPU |
| `--driver-config-json` | Конфигурация конкретного драйвера |

Оператор Gateway задаёт общие ограничения драйвера, например PID limit
sandbox, в `gateway.toml`. Параметра `--pids-limit` для отдельного sandbox нет.
CLI не задаёт память по умолчанию: укажите `--memory` или используйте общий
workload template.

Пример:

```bash
quarshell sandbox create --name worker --from cursor --cpu 2 --memory 4Gi --gpu 1
```
