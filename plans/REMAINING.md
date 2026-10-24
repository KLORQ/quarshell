# Quarshell: review и остаток переноса OpenShell

Дата сверки: 2026-10-24. Это актуальный инженерный план; подробные старые
записи и исходный scope review сохранены в `plans/openshell-go-2026-10-21/`.
Baseline поведения: OpenShell `v0.1.4`, SHA
`5a7728c304b5528e954e9ae900cf21f84e1a12b1`. Совпадение схемы или имени RPC не
считается доказательством функциональной совместимости.

## Что подтверждено сейчас

- Десять самостоятельных Go-модулей синхронизированы в `tools/modules.lock`;
  UI, Python SDK и сайт документации — отдельные репозитории.
- GitHub `main` Quarshell теперь является default branch. Root CI на commit
  `2dc5d68` прошёл: Linux, macOS, Windows, secret scan, Docker/Podman E2E,
  Gateway image, docs и hosted Linux KVM. Standalone Driver `v0.2.10` CI,
  rootful/rootless Podman и полный KVM lifecycle с Docker/Podman также прошли:
  [KVM run](https://github.com/KLORQ/quarshell-driver/actions/runs/40086627788),
  [root platform run](https://github.com/KLORQ/quarshell/actions/runs/40086645504).
- Последняя локальная проверка перед этим review: полный `task test`,
  `task ci:pins`, `task workspace:check` и KVM script contract прошли.
- Docker/Podman имеют живые Linux acceptance. VM lifecycle подтверждён на
  GitHub KVM и локальном macOS/HVF. Это закрывает старые записи «KVM acceptance
  не подтверждён», но не означает полного VM parity.

## Архитектура и владельцы

| Компонент | Ответственность |
|---|---|
| `quarshell-core` | Общие доменные типы, credential primitives и контракты изоляции. |
| `quarshell-policy` | Парсер, validation, evaluator, provider profiles и prover. |
| `quarshell-proto` | Protocol source, OpenShell compatibility adapters и Go/TypeScript bindings. |
| `quarshell-driver` | Compute SPI, Docker/Podman/Kubernetes и host isolation/supervisor. |
| `quarshell-driver-vm` | MicroVM lifecycle, KVM/HVF worker и guest transport. |
| `quarshell-gateway` | Единственный Go backend: API, auth, orchestration и durable state. |
| `quarshell-runtime` | Guest init/supervisor, SSH и runtime hardening. |
| `quarshell-proxy` | Egress enforcement и credential mediation. |
| `quarshell-sdk` / `quarshell-cli` | Публичный Go client и CLI/TUI. |
| UI / Python SDK / Docs | TypeScript/React UI, Python client и MkDocs сайт в отдельных repos. |

Корневой `KLORQ/quarshell` всё ещё содержит копии продуктовых модулей; отдельно
существует `KLORQ/quarshell-workspace` для tooling/plans. Это дублирование нужно
устранить после проверки, что workspace воспроизводит checkout, тестирование,
инвентаризацию и release-процессы без монорепозитория.

## Подтверждённые gaps

### 1. Gateway RPC и security contract — P1

`rpc_fixture_test.go` фиксирует статус всех публичных RPC, но это только
проверка fail-closed поведения и регистрации, не функциональная parity-матрица.
В status fixtures остаются `Unimplemented` для `ExchangeProviderSubjectToken`,
`PeerReportProviderReadiness`, `PeerReportEndpointStatus`,
`PeerGetSandboxProviderStatus` и `IssueSandboxToken`; peer relay/report и часть
runtime, SSH, token и stream paths требуют отдельных сценариев. Успешные
fixtures должны проверять mutation, persistence, auth principal и ошибки.

Общий `QUARSHELL_TOKEN` и статический `gateway-client` не заменяют upstream
per-user/sandbox claims, scope authorization, TLS и rotation/replay semantics.
Configured OAuth subject-token exchange теперь требует sandbox principal,
совпадающий `sandbox_id` и активные generation/credential epoch; истёкший
provider credential не обменивается. Это проверено transport regression tests,
но не заменяет валидацию JWT-SVID доверенным OAuth issuer. Без OAuth client метод
остаётся fail-closed. Сначала
зафиксировать RPC-level acceptance для identity, scope, expiry/revoke,
idempotency и credential non-disclosure; затем реализовывать RPC группами,
удаляя stub только вместе с успешным functional fixture.

Локальный `ReportProviderReadiness` теперь требует observation с положительным
sequence и session ID текущего supervisor, сверяет durable lease и не переводит
обычное startup-waiting состояние в terminal failure. Явные credential/policy/
process installation failures остаются terminal для данной revision. Durable
хранение/монотонность observation sequence, TTL, идентичность attachment/policy
revision и peer forwarding ещё не реализованы; одноразовая session-проверка не
закрывает эти acceptance rows.

### 2. Multi-Gateway и durable streams — P1

Есть replica redirect, DNS SRV discovery, fencing, durable cursors и replay.
Остаток — live rollout минимум двух Gateway с общим durable store и
переподключением клиента между replica. Граница отправки frame/сохранения
cursor сейчас даёт at-least-once и может повторить последний frame. Зафиксировать
контракт (exactly-once либо явный dedupe cursor), проверить crash/partition,
ownership takeover, cross-replica peer RPC и общую lease storage.

### 3. Linux isolation и host network boundary — P1

Работают restricted child, namespaces, Landlock, cgroup limits, no-new-privs и
baseline seccomp; открытые пункты исходного review — seccomp notification
broker, точный учёт/завершение потомков и полноценная policy mediation на host.
Закрыть raw socket/seccomp, DNS, Unix socket и host-gateway bypass сценарии на
Linux host; проверять denial и отсутствие side effects, не только API-ответ.

### 4. Backend parity — P1/P2

- Docker/Podman: базовая lifecycle, rootful/rootless и egress acceptance зелёные.
  Остались host-level socket/DNS/UDS bypass, полный outage/recovery contract и
  проверка всех 54 config fields; текущая матрица помечает 49 из 54 fields как
  `partial`.
- Kubernetes: есть adapter и config plumbing. Нужен реальный cluster lane для
  RBAC, namespace ownership, NetworkPolicy, exec/log/watch, cleanup, storage,
  credential delivery и GPU/CDI, включая rollback при ошибках между этапами.
- Windows MXC: есть build/capability probe, но нет полноценного driver/runtime
  acceptance. Требуются поддерживаемый Windows Insider host, MXC SDK/runtime,
  process/session isolation и lifecycle matrix.
- GPU/VFIO: явно unsupported до появления физического совместимого устройства;
  нужны allocation, isolation и crash/recovery проверки.

### 5. VM parity — P1/P2

Полный KVM workflow теперь проходит, локальный HVF lifecycle тоже подтверждён.
Остаются OpenShell rows с partial статусом: полноценный multi-Gateway rollout,
at-least-once cursor boundary, все credential rotation/replay ветки, lifecycle
upgrade/install rollback и оставшиеся Gateway/runtime acceptance сценарии.
Источник точных строк —
[`vm/testdata/openshell-v0.1.4-parity.json`](../quarshell-driver-vm/vm/testdata/openshell-v0.1.4-parity.json);
сводка evidence — [VM reports](evidence/README.md). Не менять parity status
только на основании package build или успешного общего CI.

### 6. CLI, SDK, bindings и prover — P2

- Сверить команды/флаги и TUI workflows с pinned OpenShell inventory; отдельно
  проверить ошибки, таймауты, вывод и cancellation.
- Закрыть Go SDK typed errors, pagination, auth refresh, cancellation и streams.
- Для Python/TypeScript выполнить curated scenarios против реального Gateway;
  generated source проверять только через закреплённую генерацию.
- Расширить symbolic prover за текущие bounded/risk/Z3 plumbing; `Inconclusive`
  и solver failure не должны превращаться в разрешение policy.
- Проверить provider profiles, examples и agent tooling по поведению, не только
  по наличию файлов.

### 7. Доказательства и release — P1/P2

Машиночитаемая `plans/evidence/matrix.yaml` по-прежнему содержит initial
`unverified` rows; README inventory ещё ссылался на удалённый Python generator.
Сначала привести matrix, `STATUS.md`, EN/RU compatibility pages и README к
последнему CI/evidence. Каждое `implemented` требует source anchor и исполняемой
проверки; `released` дополнительно требует published-consumer install.

Остались отдельные внешние release gates: опубликованный consumer без local
replace, чистая установка/upgrade/rollback, registry image publication и
подпись/notarization macOS. Реальные hardware gates — MXC и GPU/VFIO — нельзя
закрывать локальными mocks.

## Порядок выполнения

1. **P0 — источник истины:** обновить evidence matrix и feature inventory;
   синхронизировать английскую/русскую compatibility документацию; удалить
   устаревшие планы и ссылки. Приёмка: `task compat:inventory`,
   `task evidence:check` и `task evidence:report-check` отражают текущий commit.
2. **P1 — security и control plane:** Gateway auth/RPC fixtures, OAuth/token
   lifecycle, peer RPC/shared lease и multi-replica rollout; закрыть Linux
   seccomp/process/network bypass gaps. Приёмка: позитивные и негативные
   functional сценарии плюс crash/restart и cross-replica runs.
3. **P1 — продуктовые backends:** закончить Docker/Podman config/network gaps,
   затем Kubernetes live cluster; закрыть оставшиеся VM parity rows только по
   отдельным acceptance cases.
4. **P2 — clients/runtime features:** CLI/TUI/SDK/Python/TS/prover parity и
   examples. Приёмка — curated upstream journeys через реальный Gateway.
5. **P2 — platform/release:** MXC и GPU/VFIO на нужном hardware; clean consumer,
   install/upgrade/rollback, images и подписи.
6. **P3 — удалить монорепо:** перенести рабочие планы и Go tooling в уже
   существующий `quarshell-workspace`. В чистом checkout уже проверены
   `checkout-modules.sh`, `task workspace:sync`, `task workspace:check`, полный
   `task test`, `task compat:inventory`, `task evidence:check` и
   `task evidence:report-check`. Docs, Python SDK и UI checkout также
   синхронизированы с Quarshell source; MkDocs strict, Python tests (23) и UI
   production build проходят. Первоначальные 48 несовпадений относились к
   workflow в неизменяемых release-тегах и отражали их исторические зависимости.
   `task ci:pins` теперь проверяет default branches всех десяти Go-модулей
   относительно `tools/modules.lock`; 17 устаревших ссылок на `quarshell-driver`
   в CLI, Gateway и Runtime обновлены до `v0.2.10`, проверка проходит. После этого
   убрать дублирующие product copies, старую `master` ветку и архивировать/
   удалить `KLORQ/quarshell` согласно ранее заданному решению о retiring
   монорепозитория.

## Гигиена репозитория

Сохранять pinned OpenShell checkout и source hashes, license/NOTICE, conformance
fixtures, VM parity manifest и evidence reports. Удалять только генерируемые
`node_modules`, `dist`, release dry-run output, IDE state и локальные базы/логи;
они должны восстанавливаться обычными build/install командами. OpenShell-названия
в schema, generated bindings и compatibility adapters — часть внешнего контракта,
их не переименовывать ради косметики.
