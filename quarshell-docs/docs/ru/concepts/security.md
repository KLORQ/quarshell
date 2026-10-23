<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Безопасность

| Граница | Доверие |
|---------|---------|
| Host Engine / Desktop | Доверенная машина оператора |
| Sandbox-контейнер | Недоверенный агент |
| Egress proxy | Доверенный sidecar |
| Store секретов gateway | Доверенный control plane |
| noVNC / display | Только loopback оператора |

## По умолчанию

- Нет `docker.sock` в sandbox
- Bind workspace с deny-list для `$HOME`, `~/.ssh`, `~/.aws`, `~/.cursor`
- Default-deny сеть; allowlist из policy + composition провайдеров
- Секреты как `quarshell:resolve:env:…` с rewrite только на egress
- Default seccomp Docker; `no-new-privileges`; `CapDrop=NET_RAW`
- Правила путей Landlock через `quarshell-init` при ABI ≥ 1 (профиль соответствует доступному ABI до V5 или V7 при ABI ≥ 7; на Desktop часто ABI 0)
- Kernel audit для отказов Landlock при ABI ≥ 7; события попадают в audit-подсистему Linux и автоматически не копируются в `quarshell logs`
- При использовании OpenShell gRPC relay на Linux supervisor отправляет в gateway OCSF-наблюдения о запуске и выходе основного процесса и обнаруженных через `/proc` потомков. При включённой настройке `[openshell.gateway.ocsf_log]` Gateway также записывает эти наблюдения как OCSF Process Activity JSONL. `SIGCHLD` и `wait4` ускоряют локальное обнаружение выхода основного и усыновлённых supervisor процессов; опрос `/proc` каждые 250 мс остаётся резервным способом обнаружения. Relay отправляет события пачками с ожиданием до 25 мс. Сетевое время и повторные попытки всё ещё не позволяют гарантировать доставку в gateway за 250 мс. Короткие процессы, которые успевает собрать их родитель, и замены через `exec` могут быть пропущены. Записывается только basename executable, без argv и переменных окружения. Это не полный аудит процессов или файловой системы.

Egress proxy один раз разрешает имя назначения, проверяет SSRF и `allowed_ips`,
затем подключается к отфильтрованному IP. При настроенном корпоративном HTTP
proxy этот IP передаётся ему как CONNECT target, поэтому повторного DNS-
разрешения имени нет. Зарезервированные, loopback и link-local адреса остаются
закрытыми даже при `allowed_ips` в политике. Сам upstream proxy остаётся частью
доверенной сетевой границы. Значение
`QUARSHELL_PROXY_CONNECT_BY_HOSTNAME=true` отклоняется при использовании upstream
proxy, так как оно обходит проверенный IP.

На Docker Desktop для macOS прокси может не получить достоверную идентичность
бинаря, открывшего соединение. Правило, зависящее от конкретного бинаря,
требует платформу с такой идентификацией; граница контейнера и ограничения
монтирования продолжают действовать.

Пользовательская backend-конфигурация по умолчанию выключена. Если оператор
включил её, named volumes допускаются только при engine labels, привязывающих их
к проверенному workspace; Quarshell записывает identity volume и проверяет её
перед повторным запуском. Host bind и image mounts недоступны при включённом
resource admission: для их источников нет доверенного label resolver. Примеры
конфигурации есть для [Docker](../providers/docker/prerequisites.md)
и [Podman](../providers/podman/prerequisites.md).

## Что ещё проверяем

После проверки безопасности в октябре 2026 года исправлены известные пути через gateway внутренней Docker-сети, действующие CONNECT-потоки после смены политики, отсутствие SSH hardening wrapper, ошибки Docker inspect, системные mount targets и возврат удалённых gateway credentials из host environment. Для исправлений есть целевые тесты. Остаются проверки на работающем backend:

- Подтвердить, что `UpdateConfig` меняет решение запущенного proxy на Docker и на rootful/rootless Podman, в том числе после перезапуска gateway или proxy.
- Проверить блокирующий маршрут к host gateway на Podman 6.x. На версиях Podman, которые не могут обеспечить это ограничение, создание sandbox с proxy отклоняется.
- Проверить Docker IPv6 на dual-stack Engine и защитить путь workspace от гонки между проверкой и bind mount.

Эти ограничения существенны для политики с немедленным отзывом доступа. Общий статус приведён в разделе [совместимости с OpenShell](../reference/openshell-compatibility.md).

## Credentials

Предпочитайте `provider create`, а не `--env`. Cursor — через `agent login`,
без инжекта `CURSOR_API_KEY`. См. [Credentials](../guides/credentials.md).

SPIFFE access tokens для token grant хранятся только в памяти proxy. Cache
ограничен 256 записями и отклоняет токены больше 64 KiB. Его ключ учитывает всю
non-secret конфигурацию grant и request target, поэтому изменение конфигурации
не использует повторно предыдущий grant. Ротация секрета при прежнем credential
key не очищает уже выданные токены немедленно: действуют expiry и revoke
semantics провайдера. Срок кеширования не превышает lifetime токена провайдера.
Если именно expiry задаёт верхнюю границу, вычитается 30 секунд, когда это
возможно; явно заданный более короткий TTL сохраняется.

## Гигиена ресурсов

Агенты без лимита и безлимитные container logs раздувают VM Docker Desktop.
Используйте `--memory`, soft defaults и ротацию json-file по умолчанию:

- [Ресурсы Docker](../providers/docker/resources.md)
- [Логи Docker](../providers/docker/logging.md)
