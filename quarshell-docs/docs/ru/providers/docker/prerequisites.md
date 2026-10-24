<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Требования Docker

## Engine

- Docker Engine 28.0+ или Docker Desktop с рабочим API-сокетом.
- Linux: `/var/run/docker.sock` или rootless user socket.
- macOS / Windows: Docker Desktop; для reclaim памяти предпочтителен текущий
  backend **Docker VMM**.
- Podman 5.x+ на Linux запускает lifecycle subset. Для текущего полного пути с
  egress proxy нужен Podman 6+: в более ранних версиях нет блокирующего маршрута
  host-gateway. Покрытие rootful и rootless режимов различается; см. таблицу
  поддержки провайдеров.

```bash
docker version
docker info
```

## Образы

Перед create нужен хотя бы один sandbox-образ:

```bash
# Локальная сборка из workspace
task runtime:image:cli          # quarshell-sandbox:local
task docker:agent:cursor        # quarshell-sandbox:cursor

# Или каталог GHCR
docker pull ghcr.io/klorq/quarshell/sandboxes/base:latest
```

Каталог и BYOC: [Справка по образам](../../reference/images.md).

## Gateway (рекомендуется)

Proxy, секреты и `--provider` требуют доступный gateway:

```bash
quarshell gateway ensure
quarshell gateway add https://127.0.0.1:17670 --local --name local
quarshell gateway select local
```

Compose gateway: `quarshell-gateway/compose/docker-compose.yml`.

## Профиль AppArmor

Docker backend принимает `app_armor_profile` в таблице driver gateway:

```toml
[openshell.drivers.docker]
app_armor_profile = "RuntimeDefault"
```

Допустимые значения: `RuntimeDefault`, `Unconfined` и
`Localhost/<profile-name>`. Для `RuntimeDefault` и `Localhost/...` Engine host
должен сообщать о поддержке AppArmor; иначе sandbox create завершится до
создания ресурсов. `Unconfined` явно отключает AppArmor confinement.

## Driver-конфигурация от пользователя

`template.driver_config` по умолчанию выключен. Включайте его для Docker только
если workload нужны специфичные для Docker mounts или CDI-устройства:

```toml
[openshell.drivers.docker]
allow_driver_config = true
```

Named volume должен существовать и иметь labels, разрешающие подключение к
workspace sandbox:

```bash
docker volume create \
  --label openshell.ai/sandbox-attachable=true \
  --label openshell.ai/sandbox-attachable-workspace=team-a \
  agent-cache
```

Политика `resource_admission` по умолчанию требует обе labels и совпадение
workspace identity. Quarshell записывает identity допущенного volume и проверяет
её перед повторным запуском. Bind и image mounts нельзя проверять через labels
engine, поэтому они запрещены, пока admission включён; bind mounts также требуют
`enable_bind_mounts = true`.

## Память Desktop

Агенты без `--memory` и безлимитные container logs раздувают VM Desktop.
Задайте soft defaults (по желанию) и оставьте ротацию логов (включена по умолчанию):

```yaml
# ~/.config/quarshell/config.yaml
defaults:
  memory: 2g
  cpu: 2
```

См. [Ресурсы и лимиты](./resources.md) и [Логирование](./logging.md).
