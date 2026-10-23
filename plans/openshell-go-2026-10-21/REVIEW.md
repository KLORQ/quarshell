# Ревью текущей реализации и готовности к полному переносу

## 1. Главный вывод

Текущий workspace — частичная Go-реализация с широким API, но с неполным runtime и неоднородной доказательной базой. Продолжать измерять готовность числом зарегистрированных RPC недостаточно. Новый план должен начинаться с функциональной спецификации OpenShell, отдельных compute/isolation контрактов и проверяемых гарантий runtime.

По требованию пользователя старый Quarshell/Quarshell не становится обязательной основой. Новые модули реализуют нужное поведение напрямую; старые ветви, fallback-конфиги, dual readers и параллельные реализации удаляются при замене соответствующего компонента. Примеры тестовых сценариев могут служить материалом спецификации, но их результаты не переносятся автоматически на новый код.

## 2. Что проверено

- Обнаружены 15 самостоятельных репозиториев, все с незакоммиченными изменениями. Проверена рабочая копия, а не только HEAD. SHA и число изменённых entries каждого репозитория записаны в `review-manifest.json`.
- Прочитаны инструкции модулей, архитектура, compatibility/release документы, реестры gaps/RPC/features и выбранные исходники policy, gateway, driver, runtime, providers, proxy, SDK/CLI.
- В upstream обнаружены 37 crates. Для каждого подготовлено назначение в новой архитектуре: `upstream-components.md`.
- Текущий RPC-реестр содержит 108 методов шести сервисов. Дополнительно sandbox backend определяет `IsolationBoundary.Exchange` и `IsolationBoundary.Mediate`: минимум 110 RPC семи сервисов должны попасть в план. Полнота API не ограничивается RPC: CLI, policy, конфиги, SDK, packaging и локальный prover имеют отдельные контракты.
- В текущем реестре 83 публичных Gateway RPC: 79 fixtures имеют статус `partial`, 4 — `missing`. Ещё 25 методов compute/credentials/extensions имеют `missing`. Это данные реестра, а не независимый прогон 108 сценариев.
- Проверена policy библиотека запуском существующих тестов и отдельным воспроизводителем. Проверены выбранные gateway contract/watch тесты; сетевой тест ограничен sandbox текущей сессии.

`source-sha256.txt` фиксирует содержимое исходников рабочей копии на момент сбора. Список хешей не означает, что каждый файл прочитан построчно. Upstream использован как локальный baseline; его полная сборка и conformance suite не запускались.

## 3. Подтверждённые ошибки и расхождения

Приоритет P1 — исправить до заявления функциональной готовности; P2 — архитектурная или эксплуатационная проблема. Приоритет не является оценкой CVSS. Потенциальные обходы изоляции без воспроизведения не объявляются установленными уязвимостями.

### R01 · P1 · Нет upstream-бюджета размера policy

**Код:** [policy/parse.go](../../quarshell-policy/corepolicy/parse.go#L14), функции `Load` и `Parse`.

`Load` читает весь файл через `os.ReadFile`; `Parse` выполняет несколько YAML-разборов без общего лимита входа. В upstream `ParseLimits::default()` задаёт 4 MiB, depth 64, 300000 events, 100000 nodes, ограничение scalar bytes/aliases/keys/sequences.

**Воспроизведение:** документ `version: 1` и комментарий длиной 4 MiB, всего 4194317 байт, успешно проходит `Parse` и `Validate`. Соответствующий upstream default parser имеет лимит 4194304 байта. Это подтверждённое расхождение; DoS-эксплуатация не проверялась.

**Исправление в новой реализации:** лимит до выделения полного буфера, единый parser budget при построении документа, запрет повторных полных разборов. У RPC могут быть дополнительные transport limits, но библиотечный parser обязан защищать CLI, файлы и все иные вызывающие пути.

**Приёмка:** boundary fixtures `limit-1/limit/limit+1`, aliases, merge keys, глубина, количество узлов и большие scalars; превышение не должно превращаться в panic или чрезмерную память. Пример `ReadBounded` в `examples/design` решает только ограничение входа, не все ограничения YAML.

### R02 · P1 · `null` стирает различие между отсутствующей и некорректной политикой

**Код:** тот же `Parse`; upstream: [openshell-policy-schema/src/lib.rs](https://github.com/NVIDIA/OpenShell/blob/5a7728c304b5528e954e9ae900cf21f84e1a12b1/crates/openshell-policy-schema/src/lib.rs#L597), `reject_null_objects`.

**Воспроизведение:** каждый из документов `version: 1` + `process: null`, `network_policies: null`, `filesystem_policy: null` успешно проходит Go `Parse` и `Validate`. Upstream явно запрещает null для этих объектов. Пользовательская ошибка может незаметно стать поведением по умолчанию.

**Исправление:** AST-декодирование с присутствием поля; отдельно моделировать absent/null/value. Вложенные объекты должны проверяться по полной upstream-таблице, а не только по трём воспроизведённым полям. Не заменять весь новый decoder набором точечных проверок этих примеров.

**Приёмка:** одинаковые YAML/JSON результаты для absent, null, empty object, false, zero, empty list, unknown field; golden corpus сверяется с закреплённым upstream.

### R03 · P1 · Закрытие platform-event канала завершает весь комбинированный watch

**Код:** [rpc_watch.go](../../quarshell-gateway/internal/httpapi/rpc_watch.go#L105), `WatchSandbox`, ветка `case result, ok := <-platformEvents`.

При `ok == false` handler возвращает nil даже при активных `FollowLogs` и `FollowStatus`. `bridgeBackendPlatformEvents` закрывает канал при EOF backend stream. Поэтому конечный backend event stream может завершить выдачу логов и статусов работающего sandbox. Это вывод по явному control flow; специальный комбинированный runtime regression в этой сессии не выполнялся.

**Исправление:** явно определить семантику закрытия каждого источника. При независимом окончании одного источника отключить его канал (`nil`), остальные продолжить. При неожиданном EOF требующего непрерывности backend — reconnect с cursor либо явная `Unavailable`, без успешного молчаливого завершения. Завершать весь watch только по заданному stop condition, отмене или определённой ошибке.

**Приёмка:** events EOF + последующий log; events EOF + status update; dropped cursor; reconnect; медленный consumer; все источники закрыты; отмена без утечки goroutines. Пример корректного fan-in приложен.

### R04 · P1 · Credential refresh не соблюдает отмену context

**Код:** [refresh/env.go](../../quarshell-runtime/refresh/env.go#L27), `FromEnv.Refresh(_ context.Context, ...)`; `ghAuthToken` использует `exec.Command` и `Run` без deadline.

При отсутствии GitHub token запускается `gh auth token`. Зависший процесс способен удерживать refresh после отмены запроса. В коде также используется неограниченный buffer для stdout/stderr. Зависание внешней команды не воспроизводилось; отсутствие механизма отмены подтверждено исходником.

**Исправление:** context проходит через весь refresh; child process получает deadline, ограниченные stdout/stderr, ограничение ожидания pipe после выхода, redacted error. Выбор бинарника и PATH определяется доверенным операторским конфигом. Для процессов с потомками нужен platform-specific group/job cleanup.

**Приёмка:** fake credential helper зависает, игнорирует TERM, порождает потомка, пишет больше лимита, возвращает секрет в stderr; результат ограничен по времени, памяти и диагностике.

### R05 · P1 · Реестр evidence противоречит рабочей копии

**Историческое evidence:** на момент этого ревью compatibility matrix и gap register находились в `quarshell-gateway/internal/httpapi/testdata/`; эти артефакты удалены при перестройке inventory. Текущие источники: [`plans/inventory/features.yaml`](../inventory/features.yaml) и [`plans/REMAINING.md`](../../REMAINING.md). Старые выводы ниже описывают состояние на дату ревью и не являются актуальным machine-checked gate.

Найдены пять ссылок на два отсутствующих файла: `remote_compute_engine_test.go` и `driver/backend_test.go`, в трёх feature rows. Текущий реестр упоминает compatibility adapter и отсутствие readiness/configuration handlers, хотя native Backend и соответствующие handlers уже присутствуют. В документации опубликованные зависимости названы падающим gate, а gap register помечает published standalone как verified.

**Исправление:** одна машиночитаемая matrix, docs и release summary строятся из неё. Evidence включает source SHA/digest рабочей копии, host/backend/kernel/toolchain, команду, exit status, дату и артефакт результата. Проверка существования файла не считается поведенческой проверкой.

**Приёмка:** битая ссылка, устаревший SHA, пустой report и skipped обязательный сценарий блокируют promotion; новый код начинает с `unverified`, даже если старый fixture был зелёным.

## 4. Подтверждённые функциональные пробелы

### G01 · P1 · Изоляция runtime существенно уже upstream

[harden.go:47](../../quarshell-runtime/internal/harden/harden.go#L47) прямо описывает seccomp posture как Docker defaults + no-new-privileges + NET_RAW drop, с in-process filter «later». Upstream имеет отдельные sandbox/isolation boundary, seccomp notification broker и provider-managed file FD injection. Landlock и закрытая proxy network полезны, но не реализуют этот набор функций.

**Работа:** самостоятельный `quarshell-isolation` с протоколом, trusted binary identity, syscall mediation, capabilities/no_new_privs, FD lifecycle и подтверждением ограничений до запуска workload. Разделить supervisor и workload credentials. Не заявлять чистый Go runtime только на основании Go-обвязки старого Rust sandbox.

### G02 · P1 · Stable placeholders и token grants не доведены end-to-end

[provider_credentials_runtime.go:180](../../quarshell-gateway/internal/httpapi/provider_credentials_runtime.go#L180) исключает credentials с `TokenGrant` или `StablePlaceholder`. При этом proxy уже содержит grant/placeholder primitives. Наличие этих primitives не доказывает рабочую цепочку gateway → supervisor → proxy.

**Работа:** policy-bound handles, provider attachment epoch, runtime installation ACK, refresh/expiry/revoke, rejection старой generation, managed files и endpoint bindings. Сохранить отказ при отсутствии binding до готовности всей цепочки; не открывать выдачу plaintext как способ закрыть gap.

### G03 · P1 · Peer RPC представлены отказами

[openshell_provider_readiness.go:456](../../quarshell-gateway/internal/httpapi/openshell_provider_readiness.go#L456): `PeerReportProviderReadiness`, `PeerReportEndpointStatus`, `PeerGetSandboxProviderStatus`, `PeerRelay` вызывают `peerRelayGapError`.

**Работа:** peer identity, session ownership, workspace authorization, bounded bidirectional relay, fencing, restart/reconnect и конечные retry budgets. Это обязательный этап полного переноса, даже если первый инженерный milestone запускается на одном gateway.

### G04 · P1 · Gateway→VM path гарантированно заканчивается ошибкой

[spec.go:107](../../quarshell-driver-vm/vm/spec.go#L107), `validateGatewayRuntime`, после успешных preflight/probe всё равно возвращает ошибку неполной host-supervisor/isolation orchestration. В `backend_capabilities.go` VM resource capabilities CPU/memory/GPU выставлены false.

**Работа:** host supervisor, authenticated host/guest boundary, worker lifecycle, resource enforcement, workspace ownership, KVM/HVF, rootfs/OCI, recovery, GPU/VFIO при upstream-поддержке. Локальный запуск VM и успешный CLI backend test не заменяют gateway E2E.

### G05 · P1 · Process monitor не гарантирует наблюдение всех потомков

[process_monitor_linux.go:20](../../quarshell-runtime/internal/app/supervisor/process_monitor_linux.go#L20) задаёт интервал 250 ms; присутствуют также SIGCHLD scan и subreaper. Процесс, созданный и собранный своим родителем между scan, может не попасть в `/proc` snapshot. Polling не является полной точкой учёта lifecycle.

**Работа:** launcher-owned main/exec events, точная finalization каждого управляемого процесса, kernel/runtime event mechanism для descendant coverage, process identity с generation/start time, loss-aware sequence и replay. Источник событий выбирается после Linux host spike; обещание «pidfd видит всех потомков» некорректно.

### G06 · P1 · Неполная configuration admission

`ReportSandboxConfiguration` действительно реализован, проверяет generation и сохраняет admission. Поэтому старый gap text «RPC unsupported» неверен. Документация отдельно признаёт отсутствие provisioning deadline enforcement и runtime reporting gaps.

**Работа:** единый автомат provisioning → configuration pending → accepted → running/failed; durable deadline, instance fence, актуальная policy hash и provider revision. Истечение срока должно инициировать cleanup, а не только менять UI status.

### G07 · P1 · Полный функциональный scope шире текущих модулей

В текущем `quarshell-driver/internal` есть Docker, Podman, Kubernetes и VM; MXC backend не обнаружен. Отдельного policy prover, соответствующего upstream Z3 containment/risk API, в проверенных продуктовых модулях не обнаружено. TUI имеется, поэтому его нельзя объявлять отсутствующим; полнота всех upstream сценариев TUI не проверена. Есть Python client и generated TS protocol, но полный curated SDK parity не установлен.

**Работа:** Windows MXC, verifier/prover, GPU/VFIO, credential drivers, extensions, provider-managed files, SDK/TUI и packaging включаются в общий план. Они не списываются в «после полного релиза».

## 5. Архитектурные проблемы

### A01 · P2 · Storage сериализует целое состояние

[store_core.go:384](../../quarshell-gateway/internal/storage/store/store_core.go#L384): `flushLocked` маршалит весь `State`; PostgreSQL обновляет одну JSONB row. `OpenPostgres` удерживает session advisory lock, намеренно разрешая одного writer.

Это не доказательство потери данных: в коде есть atomic file replacement, locking и rollback paths. Но модель ограничивает concurrency, размер состояния и active-active gateway. Для новой реализации нужны entity repositories, транзакции, unique constraints, CAS, durable operation log и outbox. Нельзя получить HA добавлением replicas к прежнему single-writer store.

### A02 · P2 · Доменные операции живут в transport package

`internal/httpapi` содержит storage orchestration, credential cloud drivers, backend wiring, policy composition, lifecycle/recovery, auth, RPC и streaming. Границы Go modules существуют, но граница transport/application внутри gateway размыта.

Переписать gateway как transport → application services → domain/ports. Один application use case обслуживает gRPC/Connect/CLI-facing API; protobuf conversion и status codes остаются снаружи. `core` освобождается от transitively протянутых proto/gRPC зависимостей.

### A03 · P2 · Нестабильное публичное именование

Текущие документы требуют только `quarshell-*`, но в working tree остаются `cmd/quarshell-init`, `go/quarshell`, `QUARSHELL_*`, `quarshell_gateway_state`, прежний root README. Это наблюдаемое несоответствие заявленному release contract; само по себе оно не доказывает ошибку runtime.

Новая реализация получает одно имя при создании модулей. Исключения фиксируются явно: upstream wire identifiers и attribution. Старые aliases/env fallbacks/DB import не нужны по запросу пользователя.

### A04 · P2 · Платформенные ограничения требуют честной матрицы

Docker/Podman host-boundary acceptance, Kubernetes, KVM/HVF, Windows MXC и GPU не взаимозаменяемы. Текущая документация правильно оставляет ряд host gates открытыми. Ещё один mock test не закрывает rootless host routing или VM guest credential isolation.

Модель готовности: feature × backend × OS/arch × privilege mode × kernel/engine version. Неподдерживаемая комбинация возвращает явный отказ до создания ресурсов; обязательная для полного scope комбинация остаётся blocker до реализации и проверки.

## 6. Что не следует ошибочно записывать в баги

- Отсутствие `runtime.LockOSThread` в `wsinit` само по себе не доказывает Landlock bypass. Проверена используемая `go-landlock v0.10.1`: библиотека имеет thread locking и `AllThreadsLandlockRestrictSelf`. Для новой реализации нужно проверить реальный ABI/build-tag путь, а не механически добавить mutex или LockOSThread.
- `DialSSRF` уже резолвит адреса и подключается к фильтрованному IP; утверждение «нет защиты от DNS rebinding» было бы неверным. Опциональный corporate CONNECT-by-hostname требует отдельной модели доверия.
- Credentials уже фильтруются по endpoint bindings, неоднозначности owner и enforcement. Переписывание не означает разрешение plaintext при неготовом механизме.
- `RequireExtension` проверяет и capability, и реализацию интерфейса. Этот принцип стоит реализовать в новом SPI; оборачивать старый Backend ради сохранения кода не требуется.
- Runtime drop errors приводят к отказу запуска; нельзя утверждать, что любой hardening failure игнорируется. Landlock best-effort является отдельной явно выбранной семантикой.
- Повторная проверка bind-mount path уменьшает race window, но API Docker с путём не позволяет обещать атомарный host-side path pinning. Это отмечено и в текущих документах.

## 7. Результаты и ограничения проверок

Существующий `go test ./policy` прошёл. Воспроизводитель R01/R02 успешно показал четыре неожиданных принятия входов. Gateway suite с selected watch tests упёрся в запрет `listen tcp 127.0.0.1:0` текущей среды; этот отказ не является обнаруженной ошибкой gateway. Сценарии без TCP listener запущены отдельно; точный результат в `verification.txt`.

Не выполнены полный workspace suite, race/fuzz на всех модулях, upstream conformance, Linux syscall tests, контейнерные/VM/GPU/Windows lanes, published consumer и release build. Эти результаты не заимствованы из прежних JSON-статусов. В плане для них предусмотрены конкретные acceptance gates.

Это основание для нового плана реализации и приоритизации. Заявлять уже достигнутое полное parity или отсутствие всех security bugs по этому ревью нельзя.
