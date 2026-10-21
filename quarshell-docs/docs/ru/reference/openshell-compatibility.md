<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Совместимость с OpenShell

Цель совместимости Quarshell — закреплённый для этой работы последний стабильный
релиз OpenShell: [v0.1.4](https://github.com/NVIDIA/OpenShell/releases/tag/v0.1.4),
commit `5a7728c304b5528e954e9ae900cf21f84e1a12b1`. Tag скачан в
`tools/upstream/openshell-v0.1.4`; `UPSTREAM_VERSION` и `UPSTREAM_SHA` задают
baseline generated schemas, RPC inventory и gap register. Это частичная цель
совместимости, а не заявление о полном parity.

Inventory содержит 83 публичных RPC `openshell.v1.OpenShell`, 12 RPC compute
driver, пять RPC credential driver и связанные middleware/interceptor services.
Проверки регистрации handlers и proto descriptors подтверждают форму API, но не
совпадение поведения каждого метода и backend. Release register ведёт evidence
и gaps по сценариям.

Versioned feature matrix связывает каждую tracked feature с backend, public API
или конфигурационной поверхностью, test references, уровнем evidence и gate
result. `implemented`, `local_test`, `target_host_test` и `released` обозначают
максимальный подтверждённый уровень; `gate_result` фиксирует pass, partial, fail
или open. `task compat:gaps` проверяет ссылки на test-файлы и upstream pin. Пока
published-consumer gate падает, ни одна строка не заявляет `released`.

Инвентарь Docker/Podman config строится по публичным driver structs из pinned
source и сверяется с локальными decoders. `value_behavior` описывает omission,
явные empty/zero значения и runtime effect для всех 54 полей: пять строк имеют
статус `verified`, 49 — `partial`; каждая содержит ссылки на source/tests.
Частичные строки перечисляют наблюдаемые отличия и оставшиеся host/runtime
gates, не заявляя parity. Среди примеров — Cautem image/socket defaults,
Podman health-check lifecycle, presence proxy boolean и rootless ID mapping.
Config corpus остаётся release blocker. Инвентарь также фиксирует
неподдерживаемые runtime-image настройки и намеренный отказ от gateway client
certificates.


## Политика совместимости

Quarshell нацелен на последний стабильный релиз OpenShell, закреплённый tag и
полным commit SHA. На 2026-10-19 upstream помечает v0.1.4 последним стабильным
релизом; движущиеся `main`/`dev` сборки исключены. Полная совместимость требует
поведенческого parity для всех перечисленных API, CLI, config, provider,
credential, policy, backend, SDK, TUI и packaging возможностей, а не только
совпадения схем. Приведённая ниже матрица остаётся частичной и блокирует релиз,
пока каждая обязательная строка не получит проходящие conformance evidence.

До этого переключения Quarshell не поддерживает обратную совместимость с
предыдущими релизами Cautem или Quarshell. Продукт выпускается только с
каноническими `quarshell-*` module/package/config/environment/resource именами.
Старые imports, aliases, dual readers, fallback settings, adoption прежних
ресурсов и миграционный adapter Cautem-to-Quarshell не входят в продукт. Для
старых установок потребуется чистая установка; нужные данные пользователь
сохраняет отдельно. Продукт не удаляет молча посторонние старые ресурсы хоста.

## Текущий статус

| Поверхность | Состояние |
|---|---|
| Wire schema | Сгенерирована из закреплённого исходника v0.1.4; drift и число методов проверяются. Имена upstream service/fields сохраняются на compatibility endpoint. |
| Public RPC handlers | Зарегистрированы. Некоторые методы явно возвращают unsupported/precondition; поведенческое покрытие неполное. |
| Docker и Podman | Есть широкое покрытие lifecycle, policy, exec, transfer и credentials. Live acceptance на целевых хостах, published binaries, host networking, ownership и восстановление после outage блокируют релиз. |
| Logs и platform watch | Gateway вызывает native Backend log и platform-event extensions. Remote compute plugins используют тот же Backend contract; Engine-to-Backend compatibility adapter удалён. Streams ограничены sandbox, но parity durable cursor и reconnect ещё открыта. |
| Provider labels | Сохраняются и возвращаются; `ListProviders.label_selector` фильтрует до pagination и отклоняет неверный selector. |
| Operator credential export | `GetProviderCredentials` требует verified mTLS peer certificate с OU `operator`; bearer auth для этого метода отклоняется. Нужны deployment и parity по lifetime/refresh. |
| Stable placeholders и credential endpoint bindings | Текущий runtime их не поддерживает. Lint сообщает `supports_stable_placeholder=false`, а `GetSandboxProviderEnvironment` возвращает `CREDENTIALS_WITHHELD` вместо plaintext до появления policy-bound metadata. Это release blocker. |
| MicroVM | Live acceptance macOS ARM64/HVF проверяет authenticated OCI import, восстановление по digest, image config, PTY/stdin/resize/Ctrl-C и driver recovery. Gateway runtime validation остаётся fail-closed; открыты Linux KVM и Gateway→VM acceptance, workspace mounts/ownership, отдельный host UID, policy status reporting и registry auth variants. |
| CLI и SDK | Проверяются dispatch верхнеуровневых команд и отдельный help для каждого sandbox leaf; help fixtures перечисляют принятые flags. Для `sandbox create` также проверяются все документированные flags, сохранение значений, canonical argv и некорректные аргументы. Остальные вложенные parser, output/timeout, Python API и standalone/published consumers остаются открытыми. |

Provider readiness receipts, status queries и session-bound observation reporting
реализованы. MCP endpoint status reporting и его durable per-sandbox snapshot
реализованы. Sandbox-configuration admission reporting реализован без
provisioning-deadline enforcement; Go-репортёры в supervisor и peer relay paths
остаются gaps. RPC inventory содержит точные методы, направления stream, auth, handler,
fixtures, тесты и evidence. Gap register содержит остаток работ:

- [RPC inventory](https://github.com/KLORQ/quarshell-gateway/blob/main/internal/httpapi/testdata/openshell_rpc_inventory.json)
- [Feature/API/test/evidence matrix](https://github.com/KLORQ/quarshell-gateway/blob/main/internal/httpapi/testdata/openshell_compatibility_matrix.json)
- [Реестр gaps](https://github.com/KLORQ/quarshell-gateway/blob/main/internal/httpapi/testdata/openshell_compatibility_gaps.json)
- [Инвентарь Docker/Podman config fields](https://github.com/KLORQ/quarshell-driver/blob/main/internal/configcompat/testdata/openshell-v0.1.4-driver-config.json)
- [Runtime acceptance и карта тестов](runtime-acceptance.md)
- [Архитектура и backend SPI](../concepts/architecture.md)

## Повторная проверка pin

Из корня workspace выполните:

```sh
task compat:pin-check
task compat:proto
task compat:gaps
```

Команды сверяют скачанный исходник, generated proto drift, RPC inventory и
записанную baseline gaps. Они не находят поведение, которое ещё не добавили в
inventory. Docker, Podman и MicroVM требуют отдельных host lanes из
[матрицы acceptance](runtime-acceptance.md).

Канонические module и package paths продукта используют имена `quarshell-*`.
Предыдущие пути Cautem не входят в поддерживаемый контракт беты; см.
[продуктовый и релизный контракт](renaming-and-release.md).
