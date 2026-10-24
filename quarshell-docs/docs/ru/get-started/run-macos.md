<!--
SPDX-FileCopyrightText: Copyright (c) 2026 the Quarshell authors
SPDX-License-Identifier: Apache-2.0
-->

# Запуск Quarshell на macOS

В этой инструкции CLI и Gateway работают на Mac, а sandbox запускается в
Docker-контейнере внутри Docker Desktop. API Quarshell пока нестабильны и
остаются в серии `0.x`.

## Требования

- macOS на Intel (`x86_64`) или Apple silicon (`arm64`); установщик выберет
  подходящий архив.
- Установленный и запущенный Docker Desktop. Инструкция — в
  [официальной документации](https://docs.docker.com/desktop/setup/install/mac-install/).
- `curl`, `sh` и shell с `~/.local/bin` в `PATH`.

Docker Desktop запускает container engine в Linux VM. CLI и Gateway Quarshell
работают на macOS, сам sandbox — в этой VM. Передайте проект через `--upload`.

## 1. Установите CLI и Gateway

```sh
curl -fsSL https://raw.githubusercontent.com/KLORQ/quarshell-cli/main/install.sh | QUARSHELL_NO_START=1 sh
```

Установщик проверяет SHA-256 checksum и ставит `quarshell`, `quarshell-gateway`
и helper-файлы в `~/.local/bin`. `QUARSHELL_NO_START=1` позволяет сначала
запустить engine, а затем выбрать backend при запуске Gateway.

Если установщик сообщил, что `~/.local/bin` отсутствует в `PATH`, добавьте в
`~/.zshrc`:

```sh
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> ~/.zshrc
source ~/.zshrc
```

Проверьте установку:

```sh
quarshell version
```

## 2. Запустите Docker Desktop

Откройте Docker Desktop и дождитесь состояния запущенного engine. В Terminal
проверьте подключение:

```sh
docker version
docker info
quarshell doctor
quarshell gateway ensure
quarshell status
```

Команда `docker` должна работать без `sudo`. Оставляйте Docker Desktop
запущенным во время работы с Quarshell.

## 3. Скачайте sandbox image

```sh
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
export QUARSHELL_USE_GHCR=1
```

Переменная переключает alias `base` на опубликованный image. Для новых окон
Terminal добавьте `export QUARSHELL_USE_GHCR=1` в `~/.zshrc`.

## 4. Создайте и запустите sandbox

Перейдите в каталог проекта и сохраните локальную политику:

```sh
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

Создайте sandbox, загрузите в него текущую папку и выполните команду внутри:

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

`connect` открывает shell; выход — `exit`. `--upload` отправляет копию файлов
во время создания sandbox, это не live mount каталога с Mac. Политика пока не
разрешает сетевые адреса. Добавьте точечные разрешения перед запуском агента,
которому нужен интернет: [сетевая политика](../guides/policy.md).

Удалите sandbox после работы:

```sh
quarshell sandbox delete demo
```

## Альтернатива: Podman

Установите Podman по [официальной инструкции](https://podman.io/docs/installation),
создайте и запустите Linux VM:

```sh
podman machine init
podman machine start
podman info
```

Выберите Podman до первого запуска локального Gateway: именно Gateway владеет
compute backend.

```sh
export QUARSHELL_GATEWAY_RUNTIME=podman
quarshell gateway ensure
```

Скачайте image командой `podman pull ghcr.io/klorq/quarshell/sandboxes/base:latest`;
оставьте `QUARSHELL_USE_GHCR=1` включённым.

## Устранение неполадок

- Gateway недоступен: выполните `quarshell gateway ensure`, затем проверьте лог
  `~/Library/Application Support/quarshell/gateway.log`.
- Не работает `docker info`: запустите Docker Desktop и проверьте выбранный
  context командой `docker context ls`.
- Не скачивается image: проверьте доступ к registry через `docker pull`.
- `quarshell doctor` не находит Docker: проверьте `docker` в том же Terminal.
