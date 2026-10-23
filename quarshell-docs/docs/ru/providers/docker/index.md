<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Провайдер Docker

**Провайдер вычислений** — движок, на котором работают песочница и её
egress-прокси. Выбирается через `QUARSHELL_DRIVER`, виден в
`quarshell status`.

| Провайдер | `QUARSHELL_DRIVER` | Статус |
|-----------|---------------------|--------|
| Docker Engine 28.0+ на Linux | `docker` | <span class="ws-badge ws-badge--ok">поддерживается, по умолчанию</span> |
| Docker Desktop на macOS/Windows | `docker` | <span class="ws-badge ws-badge--ok">beta</span> |
| Podman 6+ на Linux, rootful/rootless | `podman` | минимум для proxy-изоляции; квалификация продолжается |
| Podman 5.x на Linux | `podman` | только lifecycle; нет изоляции proxy host-gateway |
| [Kubernetes](../kubernetes.md) | `kubernetes` | <span class="ws-badge ws-badge--soon">скоро</span> |
| [MicroVM](../microvm.md) | `vm` | <span class="ws-badge ws-badge--soon">скоро</span> |

Docker — провайдер по умолчанию. quarshell обращается к локальному Engine API
(`DOCKER_HOST` / сокет Desktop), создаёт внутреннюю сеть на sandbox, поднимает
egress-sidecar, затем sandbox-контейнер.

Минимальная версия Docker Engine — 28.0. [В инструкции OpenShell по установке]
указаны Docker 28.0+ и Podman 5.x. Наш gateway E2E проходит на Docker 28.0,
Driver container suite — на 28.0.4. OpenShell принимает Podman 5.x для общего
контракта; наш обязательный proxy/Gateway lane теперь использует Podman 6.1.4,
поскольку изоляция host-gateway проверяется через blackhole route. Rootful и
rootless квалификация 6.1.4 настроена в CI; после изменения runner требуется
новый успешный прогон. Для Docker Desktop
нужен прямой smoke на каждом поддерживаемом релизе хоста. Доступность gateway
из WSL2 описана ниже, но пока не входит в обязательную release matrix.
Kubernetes и MicroVM не являются production-провайдерами этого релиза.

[Инструкция OpenShell по установке]: https://github.com/NVIDIA/OpenShell/blob/main/docs/about/installation.mdx

## Схема (один sandbox)

```text
host Docker Engine
├── quarshell-<name>              sandbox (образ агента)
├── quarshell-proxy-<name>        egress sidecar (slim base)
├── quarshell-net-<name>          внутренняя сеть
├── quarshell-data-<name>         опциональный data volume
└── quarshell-ca-<name>           MITM CA (если proxy включён)
```

## Активация

```bash
unset QUARSHELL_DRIVER          # или: export QUARSHELL_DRIVER=docker
quarshell doctor
quarshell status
```

В статусе ожидайте `driver: docker`.

## Приватные registry

После `docker login registry.example` при загрузке образа используется Docker
CLI credential config пользователя, от имени которого работает gateway,
включая настроенные credential helpers. Данные отправляются только вместе с
запросом Engine на загрузку образа и не попадают в конфигурацию sandbox или env.

## HTTPS с частным CA

Укажите `egress_ca_bundle` в конфигурации compute-драйвера Docker и передайте
PEM bundle с доверенными корнями для проверяемых HTTPS endpoints. Он дополняет
системное хранилище; проверка цепочки и hostname остаётся включённой. Это
отдельная настройка от `proxy_ca_bundle`, который задаёт CA для HTTPS
корпоративного forward proxy. После ротации bundle пересоздайте proxy sandbox.

Если после перезапуска rootless-движка меняется UID/GID mapping, в конфигурации
compute-драйвера Docker можно включить `reconcile_data_ownership`. При включённой
настройке привилегированный init при запуске переназначит владельца файлов в
отдельном persistent volume `/quarshell/data` на пользователя sandbox. По умолчанию
настройка выключена, на больших volume операция может занять время; bind mount
workspace не затрагивается.

### Docker Desktop в WSL2

Gateway заменяет loopback listener на `host.docker.internal` для supervisor в
sandbox. Если контейнер в WSL2 всё ещё не подключается к gateway, задайте
`grpc_endpoint` явно в конфигурации Docker compute-драйвера, например
`https://host.docker.internal:17670`. Для HTTPS одновременно задайте
`guest_tls_ca`, `guest_tls_cert` и `guest_tls_key`. Endpoint должен быть доступен
из сети контейнеров Docker Desktop.

!!! tip "Не путать с credential-провайдерами"
    `--provider github`, `--provider cursor` и другие подключают к песочнице
    секреты и egress-правила. Они работают с любым провайдером вычислений —
    см. [Credential-провайдеры](../../guides/credentials.md).
