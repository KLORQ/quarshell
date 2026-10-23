<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Пересоздание sandbox

Пересоздавайте sandbox, когда policy, bind proxy, LogConfig или лимиты ресурсов
должны примениться с чистого create. У running-контейнеров HostConfig
фиксируется на create.

## Что удаляет delete

| Удаляется | Остаётся |
|-----------|----------|
| `quarshell-<name>` | Секреты провайдеров в gateway |
| `quarshell-proxy-<name>` | Записи registry до явного delete |
| `quarshell-net-<name>` | |
| volume `quarshell-data-<name>` | |
| volume `quarshell-ca-<name>` | |

Удаление data volume сбрасывает `agent login` Cursor.

## Порядок

```bash
quarshell sandbox delete cursor

quarshell gateway ensure
quarshell doctor
quarshell provider list

quarshell sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy quarshell-cli/policies/cursor-github-push-quarshell.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g

quarshell sandbox connect cursor -- agent login
quarshell logs cursor --tail --source proxy
```

## Hot reload vs recreate

| Изменение | Предпочтительно |
|-----------|-----------------|
| Узкое правило policy | `quarshell policy set … --wait` / rule approve |
| Новые memory / образ / загружаемые файлы | Recreate |
| Ротация секрета в store | `provider refresh` (часто достаточно) |
