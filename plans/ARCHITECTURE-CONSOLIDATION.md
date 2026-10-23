# Quarshell: сокращение модулей и проверка архитектуры

Дата обновления: 2026-10-23. Статус: целевая схема реализована в рабочем дереве;
перенос `tools/` на Go и переименование Cautem → Quarshell завершены. Остаются
внешние gates: синхронизация standalone snapshots, выпуск тегов `v0.x.y` и
Docker/Podman/VM/UI acceptance на целевых хостах.

## Итоговая схема

| Go-модуль | Ответственность и перенесённые части |
| --- | --- |
| `quarshell-proto` | RPC контракты и generated Go/TypeScript bindings; содержит OpenShell compatibility adapters. |
| `quarshell-core` | Domain types, credential primitives, общие ports, изоляционные контракты, logging helpers. Не зависит от proto. |
| `quarshell-policy` | Policy parser/evaluator, provider profiles, prover, provider composition и policy conformance. |
| `quarshell-driver` | Compute SPI, Docker/Podman/Kubernetes, host isolation, isolation child и host supervisor. |
| `quarshell-driver-vm` | MicroVM, worker, guest transport, KVM/HVF и VM lifecycle. |
| `quarshell-gateway` | Единственный Go backend для CLI/UI: API, OIDC, orchestration, state и credential storage/exchange adapters. |
| `quarshell-runtime` | Guest init/supervisor, hardening, sshd и runtime images/helpers. |
| `quarshell-proxy` | Egress enforcement, credential mediation и аудит. |
| `quarshell-sdk` | Публичный Go client Gateway RPC. |
| `quarshell-cli` | `quarshell` commands, локальный UX и GUI helpers. |

UI остаётся TypeScript/React приложением, Gateway предоставляет его Go API.
Убраны отдельные Go module границы display, slogx, credentials, SPI,
isolation, supervisor, prover, providers, telemetry и MXC placeholder. Их
исходники перемещены в целевые модули, а не исключены из workspace. Пустые
MXC/telemetry module skeletons помещены в `plans/archive/modules/`.
Итог workspace: **10 Go-модулей**, без циклов.

## Выполненные архитектурные шаги

1. Перенос OpenShell protobuf adapters из Core в `quarshell-proto/compat`; Core теперь не импортирует proto.
2. Объединение policy, provider profiles, prover и повторно используемых policy packages в `quarshell-policy`.
3. Перенос compute SPI, host isolation, supervisor и нейтральных SPI ports в Core/Driver; guest runtime оставлен в Runtime.
4. Объединение display с CLI, slogx и общих credential primitives с Core, credential adapters с Gateway.
5. Сведение workspace, module lock и dependency DAG к десяти модулям. `task workspace:check` и `task dag:strict` проходят.
6. Переименование активных Cautem product identifiers, команд, путей и API в Quarshell; pinned OpenShell source и его исторические source hash не изменялись.

Устаревшие отдельные imports удалены, старые Go API paths forwarding packages не
сохраняют: публичный API нестабилен, версия будет повышена в следующем `0.x`
релизе. MIT license logging package сохранена в `quarshell-core/slogx/LICENSE`.

## Проверки

- `task workspace:check`, `task dag:strict` — PASS; 10 модулей, 24 ребра, циклов нет.
- `task check` — PASS: fmt, lint, vet/vuln, conformance, pins, evidence, gateway config и тесты десяти Go-модулей.
- `task test` — PASS по всем десяти Go-модулям после переносов.
- `task standalone:strict` — PASS: все десять модулей собираются с `GOWORK=off` через локальный module proxy.
- `task docs:strict` — PASS: MkDocs собирает EN/RU без предупреждений.
- Новая Go-команда `driver-config` проверяет закреплённые Rust поля Docker/Podman, field types, ссылки на tests/evidence и целостность behavior notes; результат: 54 backend fields, 49 partial, 5 verified.
- `tools/release-dry-run.sh` собирает 15 бинарников (CLI, Gateway, Prover) для пяти целей; `release-manifest` и `release-verify` проверяют manifest, SHA256SUMS, SBOM и архивы.
- Осталось выполнить Docker/Podman/VM acceptance, UI E2E и синхронизацию standalone snapshots.

## Остаток до закрытия

1. Обновить активные build/acceptance/license scripts и документацию module map; в feature inventory сохранить смысл владельцев после схлопывания модулей. *(Перенос `tools/*.py` и release-скриптов выполнен; module map и feature inventory обновлены.)*
2. Уточнить и повысить версии изменённых зависимых модулей монотонными `v0.x.y` тегами; не использовать стабильную версию.
3. Синхронизировать десять standalone repository snapshots, проверить их с `GOWORK=off`, создать чистые initial histories без co-author или AI упоминаний и не удалять/архивировать исходный workspace до прохождения релизных gates.
4. Выполнить Docker/Podman/VM acceptance, UI `npm run check`, `npm run build`, E2E и MkDocs strict build.

## Перенос `tools/` на Go

Все общие команды доступны через `go run tools/cmd/quarshell-tools/*.go <command>`.
Перенесены workspace, pins, DAG, evidence, conformance, inventory, standalone,
backend/Gateway/driver config, capture и release manifest/verification.
Python-инструментов в `tools/` больше нет: `generate-openshell-inventory.py`
заменён командой `inventory`, а release heredoc — командами `release-manifest`
и `release-verify`.

Python SDK, protobuf code generation, MkDocs hooks, UI E2E и VM acceptance
scripts не входят в перенос `tools/` и имеют отдельные среды исполнения.
