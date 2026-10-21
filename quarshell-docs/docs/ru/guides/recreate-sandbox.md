<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Пересоздание sandbox

Пересоздавайте sandbox, когда policy, bind proxy, LogConfig или лимиты ресурсов
должны примениться с чистого create. У running-контейнеров HostConfig
фиксируется на create.

## Что удаляет delete

| Удаляется | Остаётся |
|-----------|----------|
| `cautem-<name>` | Секреты провайдеров в gateway |
| `quarshell-proxy-<name>` | Записи registry до явного delete |
| `cautem-net-<name>` | |
| volume `cautem-data-<name>` | |
| volume `cautem-ca-<name>` | |

Удаление data volume сбрасывает `agent login` Cursor.

## Порядок

```bash
cautem sandbox delete cursor

cautem gateway ensure
cautem doctor
cautem provider list

cautem sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy quarshell-cli/policies/cursor-github-push-cautem.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g

cautem sandbox connect cursor -- agent login
cautem logs cursor --tail --source proxy
```

## Hot reload vs recreate

| Изменение | Предпочтительно |
|-----------|-----------------|
| Узкое правило policy | `cautem policy set … --wait` / rule approve |
| Новые memory / образ / загружаемые файлы | Recreate |
| Ротация секрета в store | `provider refresh` (часто достаточно) |
