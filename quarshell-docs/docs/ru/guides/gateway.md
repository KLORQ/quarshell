<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Gateway

Gateway — опциональный control plane: encrypted store секретов, registry
sandbox, observation log rings и workflow policy proposals.

## Запуск

```bash
quarshell gateway ensure
quarshell gateway add https://127.0.0.1:17670 --local --name local
quarshell gateway select local
quarshell gateway info
```

Compose: `quarshell-gateway/compose/docker-compose.yml`.

Версию сборки можно посмотреть командой `quarshell-gateway --version` или
`-V`. Release image сообщает опубликованную версию; локальная сборка выводит
`0.0.0-dev`, если версия не задана при линковке.

### Обновление durable state

При запуске Gateway мигрирует схему durable store. Schema v2 добавляет стабильные
ID service endpoint и сохраняет domain setting; schema v3 сохраняет authorization
mode и назначает старым записям безопасный `strip`. Для старых записей также
сохраняется прежнее поведение с включённым domain. Перед обновлением сделайте
backup `state.json` либо одновременно PostgreSQL и каталог состояния
`$XDG_STATE_HOME/openshell` (или `$HOME/.local/state/openshell`). Gateway schema v2
отклоняет store schema v3, поэтому для rollback нужно восстановить backup до
обновления.

Адрес по умолчанию: `127.0.0.1:17670`.
Если установить `[openshell.gateway.auth] allow_unauthenticated_users = true`
или передать `--allow-unauthenticated-users`, Gateway принимает
неаутентифицированные запросы на указанном listener и пишет предупреждение.
Используйте это только в доверенной среде разработки.

## Зоны ответственности

| Поверхность | Роль |
|-------------|------|
| Secrets | AES-GCM store; KEK через `QUARSHELL_SECRETS_KEK` или `secrets.kek` |
| Providers | Именованные инстансы + метаданные composition |
| Sandboxes | Управление реестром и жизненным циклом через RPC; ограниченный буфер логов |
| Sidecar | Resolve секретов через `host.quarshell.internal` |

### Supervisor replicas

Для нескольких Gateway задайте ID, совпадающий с именем pod. В Kubernetes
используйте SRV discovery через headless Service (Helm chart настраивает его
автоматически):

Для нескольких pod также задайте общий PostgreSQL через `database.existingSecret`
и отключите файловый PVC (`persistence.enabled=false`). Локальная SQLite база
не разделяется между репликами.

```bash
QUARSHELL_GATEWAY_REPLICA_ID=gateway-0 \
QUARSHELL_GATEWAY_REPLICA_DISCOVERY_SRV='_grpc._tcp.gateway-headless.namespace.svc.cluster.local' \
quarshell-gateway
```

Ring использует 128 виртуальных узлов на реплику и SHA-256, совместимый с
OpenShell. Gateway отправляет `SessionRedirect` до регистрации локального
supervisor owner. При shutdown реплика перенаправляет новые подключения на
другую реплику. SRV membership перечитывается каждые пять секунд; при ошибке
DNS сохраняется последний корректный ring. В других средах можно указать
одинаковый статический `QUARSHELL_GATEWAY_REPLICA_PEERS` на всех репликах.
Если routing не настроен, Gateway работает в одноузловом режиме.

## Справка по HTTP API

[Спецификация OpenAPI](https://github.com/KLORQ/quarshell/blob/main/quarshell-gateway/api/openapi.yaml)
описывает только health и bootstrap-аутентификацию. Операции управления
используют закреплённый OpenShell RPC контракт. Потоки
SSH и supervisor используют OpenShell RPC `ForwardTcp`, `ConnectSupervisor` и
`RelayStream`. Browser-facing service URLs маршрутизируются по host.

URL сервиса имеет вид `workspace--sandbox--service.<domain>`. Домен по умолчанию
`openshell.localhost`; wildcard имена из `[openshell.gateway].server_sans`
добавляют custom service domains, сохраняя default. На loopback Gateway по
умолчанию использует HTTP для service URLs даже при включённом TLS. Установите
`enable_loopback_service_http = false`, чтобы требовать HTTPS на loopback.

## Клиенты управления

CLI, Go и Python SDK, а также browser console используют native OpenShell
Gateway RPC для sandbox lifecycle, списка, логов, сервисов, workspace, provider
profiles, policy, settings, identity, SSH-сессий и выполнения команд.
`logs --all` перечисляет доступные пользователю workspace и одновременно
открывает не более 24 потоков. Native gRPC доступен на отдельном gRPC listener.
Сгенерированный TypeScript-клиент использует OpenShell Connect transport на
HTTP listener Gateway; оба транспорта вызывают одни RPC handlers.

Сейчас Go Gateway проверяет management RPC по настроенному статическому Bearer
token. Он пока не проверяет OIDC access tokens и не преобразует role/scope
claims в RPC permissions. Поэтому browser PKCE login сам по себе не
аутентифицирует запросы к Gateway. Не помещайте Gateway operator token или
sandbox supervisor token в browser code.

## Когда обязателен

| Операция | Gateway |
|----------|---------|
| Создание, lifecycle, exec и upload sandbox | Обязателен |
| Attach `--provider` | Обязателен |
| Egress proxy + rewrite | Обязателен |

## Конфиг

Gateways пишутся в `~/.config/quarshell/config.yaml`:

```yaml
current: local
gateways:
  local:
    url: https://127.0.0.1:17670
```

Для local Gateway CLI читает `auth_token` из настроенного `data_dir`; Gateway
не выдаёт токены через HTTP login endpoint. CLI может хранить OIDC- и
edge-учётные данные для клиентских flow, но Go Gateway для management RPC пока
принимает настроенный bearer token. Edge Gateway использует подтверждение через
`/auth/connect` и передаёт gRPC через WebSocket `/_ws_tunnel`.

## Связанное

- [Credentials](./credentials.md)
- [Политика](./policy.md)
- Compose-файлы: `quarshell-gateway/compose/`

## Gateway TOML OpenShell

Daemon читает `--config gateway.toml`, затем `OPENSHELL_GATEWAY_CONFIG`,
затем необязательный `$XDG_CONFIG_HOME/openshell/gateway.toml`
(по умолчанию `~/.config/openshell/gateway.toml`). Явно указанный отсутствующий
файл вызывает ошибку. Для поддержанных settings приоритет: flag > env > file.

Запустите `quarshell-gateway config preflight [--path gateway.toml]`, чтобы
проверить выбранную конфигурацию без подключения к базе, создания state и
запуска listeners. Флаги daemon можно проверить после `--`, например:
`quarshell-gateway config preflight -- --port 17670`. Go preflight проверяет
загрузку конфигурации, выбор driver, связи auth/TLS и rate-limit аргументы
Gateway. Docker, Podman и Kubernetes проверяют deployment settings через
side-effect-free backend validators. VM preflight также проверяет наличие
`mke2fs` (или `mkfs.ext4`), `debugfs` и `e2fsck` с версией e2fsprogs не ниже
1.43. Ограниченные по времени вызовы `-V` выполняются только явной командой
preflight.

Startup применяет адрес основного listener, имя установки в логах,
простые log levels, SSH session TTL, local auth, OIDC/JWKS, подпись и discovery
extension JWT, TLS certificates, external SNI certificates, mTLS identity и
выбор builtin/user `provider_profile_sources`. Выбор compute driver поддерживает
только singular `compute_driver` в TOML, `--compute-driver` и
`OPENSHELL_COMPUTE_DRIVER`. Deprecated `OPENSHELL_DRIVERS` принимает одно
имя и выводит migration warning. Flag и canonical environment имеют приоритет;
конфликтующие selectors отклоняются. Referenced paths сохраняются буквально и
вычисляются относительно рабочего каталога процесса. `disable_tls=true`
игнорирует server certificates; одновременно заданный client CA отклоняется.
Для remote compute driver на Unix socket укажите `--compute-driver-socket`
или `OPENSHELL_COMPUTE_DRIVER_SOCKET` вместе с canonical driver flag или
environment variable; это заменяет встроенный backend с таким именем.

Поддержка частичная. In-process VM backend применяет `state_dir`,
`default_image`, `bootstrap_image`, `vcpus`, `mem_mib` и `overlay_disk_mib`;
`rootfs_tar_staging_dir`, `rootfs_tar_max_bytes`, `allow_driver_config`
(по умолчанию false), `krun_log_level` (по умолчанию 1; значения 0–5 — от
off до trace), а также гостевые overrides `sandbox_uid` и
`sandbox_gid`. Если задан один из UID/GID overrides, VM обновляет учётную
запись `sandbox` в writable overlay, поэтому workload может разрешать её по
имени. Для HTTPS Gateway endpoint VM использует только `guest_tls_ca` и
передаёт CA host-side guest relay; `guest_tls_cert` и `guest_tls_key`
отклоняются, как в OpenShell. Относительный `state_dir` разрешается от
рабочего каталога процесса.
Gateway передаёт VM backend значения `default_image`, `state_dir`,
`krun_log_level`, vCPU, memory и overlay вместе с defaults OpenShell. Их
environment variables не переопределяют значения Gateway. Environment fallback
используется для bootstrap image, TLS CA, UID/GID и rootfs tar, если нет
соответствующего deployment field. `OPENSHELL_VM_RUNTIME_DIR` и
`OPENSHELL_VM_IMAGE_PULL_CONCURRENCY` читаются напрямую; layer pull concurrency
использует upstream default 4 и максимум 16. Пустой `default_image` допустим,
только если задан `bootstrap_image`.
Host supervisor binary overrides и включённые proxy, SPIFFE и GPU завершаются
ошибкой, пока эти backend-функции не перенесены. Явное `false` в boolean
feature flags принимается как no-op. Значения GPU memory/vCPU валидируются,
но игнорируются, пока GPU отключён.
Неподдержанные VM deployment поля
отклоняются. Другие driver-specific runtime
settings, конфигурация supervisor OTLP и provider profile sources на основе
interceptors ещё не портированы. Provider readiness receipts, status queries и
session-bound observations от supervisor поддерживаются; остаются multi-replica
маршрутизация peer relay и Go-репортёры в supervisor, а также полное
соответствие readiness/metrics и provisioning deadlines. Настроенные supervisor middleware и Gateway interceptor
endpoints поддерживаются. Локальные и remote compute drivers уже работают. Неподдержанные settings блокируют startup
до создания state и открытия listeners. Loader также отклоняет unknown/duplicate
keys, отсутствующие required fields, неверные enums и database URL внутри TOML.
Parser принимает schema version 2 из закреплённого v0.1.4 и его singular
selector `compute_driver`; legacy список `compute_drivers` отклоняется как
неизвестное поле. Парсинг upstream fixture не означает, что
все настройки его drivers уже запускаются: неподдержанные backend settings
по-прежнему fail closed на startup.
Без файла OpenShell пока действуют прежние defaults quarshell.
Необязательные `health_bind_address` и `metrics_bind_address`, а также
`OPENSHELL_HEALTH_PORT` / `OPENSHELL_METRICS_PORT` и соответствующие флаги
запускают отдельные listeners. `/healthz` — liveness check с пустым ответом;
`/readyz` показывает cached readiness базы данных. Metrics listener
экспортирует Prometheus metrics запросов, relay, readiness и quota.
Общий gRPC rate limit принимает `grpc_rate_limit_requests` вместе с
`grpc_rate_limit_window_seconds`; также поддерживаются соответствующие
переменные `OPENSHELL_GRPC_RATE_LIMIT_*` и флаги `--grpc-rate-limit-*`. Для
включения оба значения должны быть положительными; ноль в любом из них
отключает лимит. Лишние запросы получают `RESOURCE_EXHAUSTED`.
Необязательная таблица `[openshell.gateway.otlp]` включает gateway HTTP traces
через OTLP/gRPC. Укажите URL collector, например
`https://otel-collector.example:4317`; `service_name` по умолчанию равен
`quarshell-gateway`. Spans содержат HTTP method, response status, request ID
и RPC service/method/status, если применимо. Пути, query, остальные headers и
bodies не отправляются.
Некорректный endpoint отключает export и записывается в Gateway log;
сам Gateway продолжает обслуживать requests.

TOML loader распознаёт OpenShell OIDC issuer, audience, JWKS и role claim, но Go
Gateway пока не подключает их к RPC authentication. Проверка OIDC signature
есть в runtime identity package, однако Gateway её не вызывает. Для текущего
management API задайте `QUARSHELL_TOKEN`; настройки OIDC пока не включают
аутентификацию Gateway.

TLS может загрузить server certificate/key pair. Если задан `--tls-client-ca`,
HTTP listener требует client certificate от этой CA. Сам certificate не
преобразуется в Gateway user или operator role, поэтому management RPC всё
равно требуют bearer token. gRPC listener использует server TLS certificate,
но не запрашивает client certificate. Параметры OpenShell `mtls_auth` и
operator-role пока не подключены к Gateway runtime. Автоматическая local PKI и
hot reload сертификатов также не реализованы.

Edge WebSocket tunnel по умолчанию выключен. Его можно включить через
`enable_websocket_tunnel = true`, `OPENSHELL_ENABLE_WEBSOCKET_TUNNEL=true` или
`--enable-websocket-tunnel true`. Туннель передаёт запросы loopback-only
HTTP/2 listener.

По умолчанию используются встроенный и сохранённый user profile catalogs.
Исходный список позволяет выбрать только нужные источники, например:

```toml
[openshell.gateway]
provider_profile_sources = [{ type = "builtin" }]
```

Interceptor source отклоняется, пока не появятся gateway interceptor runtime
и его catalog protocol.
