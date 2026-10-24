<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Cursor Agent

## Образ

```bash
task docker:agent:cursor
# или: docker pull ghcr.io/klorq/quarshell/sandboxes/cursor:latest
```

## Провайдеры и policy

```bash
CURSOR_API_KEY=… quarshell provider create --name cursor --type cursor --credential CURSOR_API_KEY
GITHUB_TOKEN=…   quarshell provider create --name gh --type github --credential GITHUB_TOKEN

ORG=YOUR_ORG
sed "s/YOUR_ORG/${ORG}/g" quarshell-cli/policies/cursor-github-push.yaml \
  > /tmp/cursor-github-push.yaml
```

Профиль `cursor` в основном даёт egress allow (`**.cursor.sh` / `**.cursor.com`).
`CURSOR_API_KEY` в guest **не** инжектится (`inject_env: false`).

Builtin `github` — в основном read/clone. Для push нужна write-capable base
policy (пример выше).

## Create

```bash
quarshell sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /tmp/cursor-github-push.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

Проверка:

```bash
quarshell sandbox exec cursor -- env | grep -E 'TOKEN|KEY|CURSOR' || true
# ожидаем: GITHUB_TOKEN=quarshell:resolve:… ; нет CURSOR_API_KEY
```

## Login и запуск

```bash
quarshell sandbox connect cursor -- agent login
quarshell sandbox connect cursor -- agent
```

OAuth пишется на persist volume (`/quarshell/data`). После `stop`/`start`
login обычно сохраняется, пока volume не удалён.

```bash
quarshell logs cursor --tail --source proxy
quarshell term
```

В TUI выберите sandbox стрелками или `j`/`k`; `enter`/`c` подключает к нему,
`e` запускает exec, `l` показывает его логи, `a` — логи всех sandbox. В live
логах `p` приостанавливает и возобновляет отображение, пока ограниченный буфер
продолжает собирать строки; `s` переключает source-фильтры активного
подключения. Локально доступны `all`, `sandbox`, `proxy`, а через Gateway —
`all`, `gateway`, `sandbox`; `r` переподключает поток. Нажмите `Esc`, чтобы
вернуться из логов, или `q`, чтобы выйти. Клавиши `↑`/`↓` или `j`/`k`
выбирают строку, `Enter` показывает её детали, `g`/`G` выбирают первую или
последнюю строку.
Нажмите `y`, чтобы отправить выбранную строку, или `Y`, чтобы отправить видимые
строки логов в clipboard терминала; поддержка зависит от терминала.
Клавиша `v` начинает или завершает цветом выделенный визуальный диапазон; перемещайтесь `j`/`k`,
а затем нажмите `y`, чтобы скопировать выбранные строки.

## Host IDE (C2)

Загруженный checkout находится в `/workspace`; рабочая папка хоста в sandbox
не монтируется. Сеть и git запускайте через sandbox:

```bash
quarshell sandbox exec cursor -- go test ./...
quarshell sandbox exec cursor -- gh repo view
```

Каталоги хоста не монтируются в Gateway sandbox. Для credentials используйте
provider profiles и загружайте только нужные workload-файлы.

## Режимы

| Режим | Смысл |
|-------|-------|
| C1 | Agent CLI внутри sandbox |
| C2 | IDE на хосте + sandboxed workspace / exec |
