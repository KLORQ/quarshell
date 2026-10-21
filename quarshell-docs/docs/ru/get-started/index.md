<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Быстрый старт

cautem — один CLI, который запускает coding-агентов в песочницах с
политикой на Docker или Podman. Установите CLI, убедитесь, что контейнерный
движок работает, и скачайте образ песочницы — затем
[запустите gateway](gateway.md) и [создайте первую песочницу](first-sandbox.md).

## Установка CLI

### через установщик <small>рекомендуется</small> { #with-installer data-toc-label="через установщик" }

Последняя опубликованная версия CLI — `v0.1.6`. Закрепите её при
установке, чтобы инструкции оставались воспроизводимыми до выхода стабильного релиза.
Откройте терминал и выполните:

```bash
curl -LsSf https://raw.githubusercontent.com/cautem/quarshell-cli/main/install.sh \
  | CAUTEM_VERSION=v0.1.6 sh
cautem version
```

Бинарь ставится в `~/.local/bin` — этот каталог должен быть в `PATH`.

Для установки в другой каталог:

```bash
curl -LsSf https://raw.githubusercontent.com/cautem/quarshell-cli/main/install.sh \
  | CAUTEM_VERSION=v0.1.6 CAUTEM_INSTALL_DIR=/usr/local/bin sh
```

### из исходников

Сборка из checkout workspace — если нужен свежий `main` или вы разрабатываете
cautem:

```bash
cd /path/to/cautem
export GOWORK=$PWD/go.work

go build -C quarshell-cli -o cautem ./cmd/cautem
./quarshell-cli/cautem install
./quarshell-cli/cautem version
```

## Контейнерный движок

Нужен работающий провайдер вычислений. По умолчанию — Docker; Podman работает
через тот же Engine API.

=== "Docker"

    Docker Engine 28.0+ или Docker Desktop. На macOS и Windows лучше backend
    **Docker VMM**.

    ```bash
    docker info
    cautem doctor
    cautem status
    ```

=== "Podman на macOS"

    ```bash
    podman machine init     # один раз
    podman machine start
    export CAUTEM_DRIVER=podman
    cautem status
    ```

=== "Podman на Linux"

    ```bash
    systemctl --user enable --now podman.socket
    export CAUTEM_DRIVER=podman
    cautem status
    ```

`cautem status` должен показать выбранный драйвер (`docker` или `podman`).
Подробнее о движках: [Docker](../providers/docker/index.md) ·
[Podman](../providers/podman/index.md).

## Образ песочницы

Скачайте базовый образ — в нём есть всё, что нужно агенту в CLI:

```bash
docker pull ghcr.io/cautem/cautem/sandboxes/base:latest
```

!!! tip "Собрать образы самому"
    Из корня workspace: `task runtime:image:cli` собирает базовый образ,
    `task docker:agent:cursor` — агента Cursor. Полный список —
    [Образы](../reference/images.md).
