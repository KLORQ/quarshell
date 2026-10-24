<!--
SPDX-FileCopyrightText: Copyright (c) 2026 the Quarshell authors
SPDX-License-Identifier: Apache-2.0
-->

# Запуск Quarshell на Linux

В инструкции CLI и Gateway работают на Linux-хосте, sandbox создаётся через
Docker Engine. API Quarshell пока нестабильны и остаются в серии `0.x`.

## Требования

- 64-битный Linux на `x86_64` или `arm64`.
- Docker Engine с доступным API socket либо Podman с API socket.
- `curl`, `sh` и shell с `~/.local/bin` в `PATH`.

Пользователь, запускающий `quarshell-gateway`, должен иметь доступ к socket
container engine. Доступ к rootful Docker socket даёт контроль над хостом,
эквивалентный root. Используйте отдельный хост и следуйте политике безопасности
вашей системы.

## 1. Установите container engine

Для Docker следуйте официальной инструкции для своего дистрибутива, например
[для Ubuntu](https://docs.docker.com/engine/install/ubuntu/). Запустите сервис
и предоставьте текущему пользователю доступ к нему. Проверьте:

```sh
docker version
docker info
docker run --rm hello-world
```

Для Podman используйте [официальную инструкцию установки](https://podman.io/docs/installation).
В системе с пользовательской сессией systemd запустите socket:

```sh
systemctl --user enable --now podman.socket
podman info
```

## 2. Установите CLI и Gateway

```sh
curl -fsSL https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh | QUARSHELL_NO_START=1 sh
```

Установщик проверяет SHA-256 checksum и ставит `quarshell`,
`quarshell-gateway` и helper-файлы в `~/.local/bin`. `QUARSHELL_NO_START=1`
позволяет выбрать и запустить engine до запуска Gateway.

Если нужно, добавьте каталог в `PATH` через `~/.profile`:

```sh
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> ~/.profile
. ~/.profile
```

Проверьте CLI и Gateway:

```sh
quarshell version
```

## 3. Выберите Docker или Podman

Локальный Gateway по умолчанию использует Docker. Чтобы выбрать Podman,
задайте backend до запуска Gateway:

```sh
export QUARSHELL_GATEWAY_RUNTIME=podman
```

Оставьте переменную в shell, из которого вызываете `quarshell`. Если Gateway уже
работает с другим backend, перезапустите его после смены настройки. Проверьте
доступ к engine от имени пользователя Gateway:

```sh
docker info   # Docker
podman info   # Podman
quarshell gateway ensure
quarshell status
```

## 4. Скачайте image и подготовьте политику

Для Docker:

```sh
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
```

Для Podman выполните `podman pull ghcr.io/klorq/quarshell/sandboxes/base:latest`.
Включите alias
`base` для опубликованного image и сохраните policy:

```sh
export QUARSHELL_USE_GHCR=1
mkdir -p ~/.config/quarshell
cat > ~/.config/quarshell/default-policy.yaml <<'EOF'
version: 1
filesystem_policy:
  include_workdir: true
  read_only:
    - /usr
    - /lib
    - /proc
    - /dev/urandom
    - /app
    - /etc
    - /var/log
    - /AGENTS.md
  read_write:
    - /tmp
    - /dev/null
    - /sandbox
    - /workspace
landlock:
  compatibility: best_effort
network_policies: {}
EOF
```

## 5. Создайте sandbox и выполните в нём команду

Запускайте команды из каталога проекта, который нужно загрузить:

```sh
quarshell sandbox create \
  --name demo \
  --gateway-workspace default \
  --from base \
  --policy ~/.config/quarshell/default-policy.yaml \
  --upload "$PWD"

quarshell sandbox list
quarshell sandbox exec demo -- uname -a
quarshell sandbox connect demo
```

`connect` открывает shell; выход — `exit`. `--upload` отправляет копию каталога
при создании, но не монтирует живую папку хоста. Политика не разрешает сетевые
адреса, пока вы не добавите точечные правила: [сетевая политика](../guides/policy.md).

Удалите sandbox после работы:

```sh
quarshell sandbox delete demo
```

## Устранение неполадок

- Gateway не проходит health check: запустите `quarshell gateway ensure` и
  проверьте `~/.config/quarshell/gateway.log`.
- `docker info` или `podman info` не работает: настройте доступ к сервису/socket
  для того же пользователя и повторите проверку.
- Image не скачивается: проверьте доступ к registry командой `pull` engine.
- Gateway игнорирует Podman: задайте `QUARSHELL_GATEWAY_RUNTIME=podman` до запуска.
  Инструкция отключает автозапуск через `QUARSHELL_NO_START=1` при установке.
