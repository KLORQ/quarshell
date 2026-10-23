<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Первая песочница

CLI установлен, gateway запущен — создайте песочницу для проекта, с которым
будет работать агент.

Чтобы alias `base` указывал на опубликованный Quarshell image, задайте
`QUARSHELL_USE_GHCR=1` и сохраните policy-файл, как показано в инструкциях для
[macOS](run-macos.md) или [Linux](run-linux.md). Ниже предполагается путь
`~/.config/quarshell/default-policy.yaml`.

## Создание

Создайте sandbox под управлением Gateway:

```bash
quarshell sandbox create \
  --name demo \
  --gateway-workspace default \
  --from base \
  --policy ~/.config/quarshell/default-policy.yaml \
  --upload "$PWD" \
  --memory 2g
```

| Флаг | Что делает |
|------|------------|
| `--name` | Имя sandbox |
| `--gateway-workspace` | Workspace выбранного Gateway |
| `--from` | Alias image; `base` использует GHCR при `QUARSHELL_USE_GHCR=1` |
| `--policy` | YAML-файл политики sandbox |
| `--upload` | Копия локального проекта в `/workspace` |
| `--memory` | Лимит памяти sandbox |

## Работа внутри

```bash
quarshell sandbox list
quarshell sandbox exec demo -- uname -a
quarshell sandbox connect demo
```

`connect` открывает интерактивный shell в sandbox. Gateway workspace задаёт
область ресурса и не монтирует локальную папку.

## Доступы

Опционально. Секреты хранятся в gateway и не попадают в песочницу — агент
видит только плейсхолдеры:

```bash
GITHUB_TOKEN=… quarshell provider create --name gh --type github --credential GITHUB_TOKEN

quarshell sandbox create \
  --name demo-with-gh \
  --gateway-workspace default \
  --from base \
  --policy ~/.config/quarshell/default-policy.yaml \
  --provider gh
```

Подробнее: [Credential-провайдеры](../guides/credentials.md) ·
[Cursor Agent](../guides/cursor.md).

## Логи, остановка, удаление

```bash
quarshell logs demo --tail --source proxy
quarshell sandbox stop demo
quarshell sandbox delete demo
```

`delete` удаляет песочницу, её proxy sidecar, сеть и data volumes. Секреты
провайдеров остаются в gateway.

## Что дальше

Когда агент упирается в заблокированный хост, он получает 403 с причиной.
В разделе [Политика](../guides/policy.md) — как одобрить узкое правило без
пересоздания песочницы, а в [Пересоздание sandbox](../guides/recreate-sandbox.md) —
какие изменения всё же требуют нового контейнера.
