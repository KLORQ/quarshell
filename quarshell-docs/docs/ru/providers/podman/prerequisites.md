<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Требования Podman

## Linux (rootless)

```bash
systemctl --user enable --now podman.socket
export QUARSHELL_DRIVER=podman
# export QUARSHELL_PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock
# или: export DOCKER_HOST=unix:///run/user/$UID/podman/podman.sock

podman info
quarshell status
```

## macOS (Podman Machine)

```bash
podman machine init     # один раз
podman machine start
export QUARSHELL_DRIVER=podman
quarshell status
```

Discovery также смотрит
`~/.local/share/containers/podman/machine/podman.sock` и `podman info`.

## Образы

Те же теги, что у Docker (`quarshell-sandbox:local`, `:cursor`, … или GHCR).
Соберите через Docker tooling с загрузкой в store Podman, либо
`podman pull` / `podman load`.

```bash
podman images | grep quarshell
```

## Gateway

Gateway по-прежнему на хосте (бинарь или compose). Выберите его до create с
proxy/providers — как для Docker.

## Профиль AppArmor

Podman backend принимает `app_armor_profile` в таблице driver gateway:

```toml
[openshell.drivers.podman]
app_armor_profile = "RuntimeDefault"
```

Допустимые значения: `RuntimeDefault`, `Unconfined` и
`Localhost/<profile-name>`. Для `RuntimeDefault` и `Localhost/...` системная
информация Podman должна сообщать о поддержке AppArmor; иначе конфигурация
завершится ошибкой до создания sandbox. `Unconfined` явно отключает AppArmor
confinement.

## Driver-конфигурация от пользователя

`template.driver_config` по умолчанию выключен. Включайте его в таблице Podman
только если workload нужны специфичные для Podman mounts или CDI-устройства:

```toml
[openshell.drivers.podman]
allow_driver_config = true
```

Named volume должен существовать и иметь стандартные admission labels для
целевого workspace:

```bash
podman volume create \
  --label openshell.ai/sandbox-attachable=true \
  --label openshell.ai/sandbox-attachable-workspace=team-a \
  agent-cache
```

Backend проверяет эти labels и identity volume перед повторным запуском. Bind и
image mounts запрещены, пока `resource_admission.enabled = true`: Podman не
предоставляет доверенный label resolver для таких источников. Для bind mounts
также требуется `enable_bind_mounts = true`.
