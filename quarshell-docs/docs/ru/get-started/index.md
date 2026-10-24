<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Быстрый старт

quarshell — один CLI, который запускает coding-агентов в песочницах с
политикой на Docker или Podman. Установите CLI, убедитесь, что контейнерный
движок работает, и скачайте образ песочницы — затем
[запустите gateway](gateway.md) и [создайте первую песочницу](first-sandbox.md).

## Установка CLI

Полная инструкция по платформе: [macOS](run-macos.md) или [Linux](run-linux.md).

### через установщик <small>рекомендуется</small> { #with-installer data-toc-label="через установщик" }

Установщик скачивает проверенный архив релиза из `quarshell-cli`, ставит CLI
и вспомогательные файлы в `~/.local/bin` и запускает локальный Gateway, если он
включён для платформы. В текущем Windows-архиве есть только CLI. Выполните:

```bash
curl -LsSf https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh | sh
quarshell version
quarshell status
```

`quarshell status` должен показать `Connected`. `QUARSHELL_NO_START=1` отключает
запуск gateway, `QUARSHELL_VERSION=vX.Y.Z` закрепляет релиз.

Для Podman сначала запустите engine и установите CLI с `QUARSHELL_NO_START=1`.
Затем экспортируйте `QUARSHELL_GATEWAY_RUNTIME=podman` до команды
`quarshell gateway ensure`, иначе установщик запустит Docker backend по умолчанию.

Бинарники и вспомогательные файлы ставятся в `~/.local/bin` — этот каталог должен быть в `PATH`.

Для установки в другой каталог:

```bash
curl -LsSf https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh \
  | QUARSHELL_INSTALL_DIR=/usr/local/bin sh
```

### из исходников

Сборка из workspace опубликованных модулей:

```bash
git clone https://github.com/KLORQ/quarshell-workspace.git
cd quarshell-workspace
tools/checkout-modules.sh
task workspace:sync

go build -C quarshell-cli -o quarshell ./cmd/quarshell
go build -C quarshell-gateway -o quarshell-gateway ./cmd/quarshell-gateway
./quarshell install
./quarshell version
```

## Контейнерный движок

Нужен работающий провайдер вычислений. По умолчанию — Docker; Podman работает
через тот же Engine API.

=== "Docker"

    Docker Engine 28.0+ или Docker Desktop. На macOS и Windows лучше backend
    **Docker VMM**.

    ```bash
    docker info
    quarshell doctor
    quarshell status
    ```

=== "Podman на macOS"

    ```bash
    podman machine init     # один раз
    podman machine start
    export QUARSHELL_GATEWAY_RUNTIME=podman
    quarshell gateway ensure
    quarshell status
    ```

=== "Podman на Linux"

    ```bash
    systemctl --user enable --now podman.socket
    export QUARSHELL_GATEWAY_RUNTIME=podman
    quarshell gateway ensure
    quarshell status
    ```

`quarshell status` должен подтвердить соединение с выбранным Gateway.
Подробнее о движках: [Docker](../providers/docker/index.md) ·
[Podman](../providers/podman/index.md).

## Образ песочницы

Скачайте базовый образ — в нём есть всё, что нужно агенту в CLI:

```bash
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
export QUARSHELL_USE_GHCR=1
```

!!! tip "Собрать образы самому"
    Из корня workspace: `task runtime:image:cli` собирает базовый образ,
    `task docker:agent:cursor` — агента Cursor. Полный список —
    [Образы](../reference/images.md).
