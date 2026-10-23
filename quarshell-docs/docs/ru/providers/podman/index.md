<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Провайдер Podman

Podman — полноценный провайдер вычислений. Gateway регистрирует его как
нативный модульный backend и использует Docker-совместимый Engine API client
для операций с контейнерами. Labels, сети, proxy sidecar и exec совпадают с
Docker.

## Активация

```bash
export QUARSHELL_DRIVER=podman
# опционально:
# export QUARSHELL_PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock

quarshell status    # driver: podman
```

## Приватные registry

Перед созданием sandbox выполните `podman login registry.example`. При загрузке
образа сначала используется подходящая запись из `REGISTRY_AUTH_FILE`,
`$XDG_RUNTIME_DIR/containers/auth.json` или `~/.config/containers/auth.json` —
в таком порядке. Для другого authfile задайте `registry_auth_file` в конфигурации
compute-драйвера Podman. Если записи для registry нет, quarshell использует Docker
CLI credential config. Credentials передаются только в запросе загрузки образа
и не копируются в sandbox.

Для проверяемых HTTPS endpoints с частным CA задайте `egress_ca_bundle` в
конфигурации compute-драйвера Podman и укажите PEM bundle. Он дополняет системное
хранилище, сохраняя проверку цепочки и hostname. Отдельный `proxy_ca_bundle`
доверяет самому HTTPS forward proxy. После ротации CA пересоздайте proxy sandbox.

Если в rootless Podman может измениться UID/GID mapping, настройку
`reconcile_data_ownership` можно включить в конфигурации compute-драйвера. Для
sandbox с persistent data при запуске тогда переназначаются файлы отдельного
volume `/quarshell/data` на пользователя sandbox. По умолчанию настройка выключена,
на больших volume операция может занять время; bind mount workspace не меняется.

Флаги ресурсов, ротация логов, slim proxy и soft defaults — как у Docker:
[Ресурсы Docker](../docker/resources.md), [Логи Docker](../docker/logging.md).
