<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Runtime acceptance и карта тестов

> Историческая пометка: номера PR, commit SHA и опубликованные версии модулей
> ниже относятся к прежней истории репозиториев и артефактам до очистки history.
> Они сохранены как аудит и не подтверждают прохождение этих lanes текущими
> исходниками или их публикацию.

Матрица описывает поведенческий corpus OpenShell v0.1.4 и backend-проверки,
которые нужны до заявления о production-ready Quarshell. Это engineering gate,
а не заявление о совместимости.

## Значения статусов

| Статус | Значение |
|---|---|
| `local tests` | Unit, protocol или integration тесты есть и проходят в текущем checkout. |
| `host gate open` | Нужна проверка настоящего engine, восстановления daemon, границы безопасности или опубликованного артефакта на целевом хосте. |
| `unsupported` | Запрос явно отклоняется или credentials удерживаются; незаметного downgrade нет. |
| `open` | Реализация или evidence поведения не завершены. |

## Corpus API и пользовательского поведения

| Группа функций | Покрытие в checkout | Тесты / evidence | Оставшаяся проверка |
|---|---|---|---|
| Публичный control protocol | Inventory закреплён на v0.1.4: 83 public RPC и связанные compute, credential, middleware и interceptor services. Наличие handler проверяет только форму. | [реестр функций](https://github.com/KLORQ/sandbox.dev/blob/main/docs/ru/reference/feature-inventory.yaml) | Golden inputs/results на каждый метод, отмена stream, metadata и совпадение ошибок остаются открытыми. |
| Создание и inspect sandbox | Gateway один раз преобразует pinned OpenShell create request в `BackendCreateRequest`; preflight, create, lifecycle, rollback и recovery напрямую вызывают зарегистрированные модульные `driver.Backend`. VM exec, process/log/transfer, policy и platform watch используют объявленные native extensions. Плоских `Spec` и `Engine` adapters больше нет. | `rpc_sandbox_test.go`, `rpc_templates_test.go`, `runtime_recovery_test.go`, `rpc_exec_test.go`, `remote_compute_backend_test.go`, `rpc_watch_test.go`; backend contract и capability проверки Driver. | Полный per-method behavior corpus pinned CLI/backend ещё открыт; нужны дополнительные проверки отмены stream и event resume. Gateway create для VM остаётся fail-closed до реализации managed host supervisor и authenticated host/guest isolation contract. |
| OCI image inputs | MicroVM принимает registry reference или staged rootfs tar. Tag разрешается в manifest digest; VM выбирает `linux/<host-arch>`, проверяет image config, digest/size compressed layers и применяет whiteout. Immutable ext4 base кешируется по source/selected manifest digest, platform, версии rootfs layout и digest guest agent. Квота кеша по умолчанию — 64 GiB; удаляются только образы без durable sandbox references и активных create leases. Для private registry используется Docker/Podman keychain; локальный acceptance проверяет Docker credential helper. OCI `Entrypoint`, `Cmd`, `Env`, `User` и `WorkingDir` применяются к in-memory launch spec; recovery повторно получает config по immutable platform manifest digest. Image env/user не попадают в durable state, reserved identity variables фильтруются, root identity отклоняется. Linux worker запускается в частных user/mount namespaces, изолирует распространение mounts и не наследует environment Gateway; namespace root отображается на host account Gateway и не является отдельным host UID. | `quarshell-driver-vm/vm/oci_image_test.go`, `quarshell-driver-vm/vm/launch_spec_test.go`, `quarshell-driver-vm/vm/live_acceptance_test.go`, `quarshell-driver-vm/vmguest/agent_linux_test.go`, `image_cache_test.go`, `rootfs_image_test.go`, `rootfs_path_test.go`, `worker_process_linux_test.go`, `vm_test.go` | Другие реализации credential helper и варианты TLS/auth на целевых host; отдельный host UID, fd-based mount allowlist, UID/GID mapping и persistence; Gateway/Linux KVM acceptance. |
| Lifecycle, restart и host↔guest protocol | Для Docker/Podman есть package и Gateway coverage lifecycle, recovery и main-process restart policy. VM control frames используют protocol v1; обе стороны отклоняют пропущенную или неизвестную версию, guest возвращает свою версию. `start-workload` ждёт закрытый supervisor readiness marker, который создаётся после старта workload; путь marker удаляется из environment дочернего процесса. Start и worker recovery вращают per-sandbox token через durable pending record; replay со старым token разрешён только для того же кандидата и rotation ID. VM guest передаёт проверенную restart policy supervisor; live acceptance на macOS ARM64 подтвердил `on-failure` restart с текущими host и guest agent. Новый VM driver может восстановить launch spec уже работающей sandbox перед adoption worker, не сохраняя image env или identity в durable state. | `quarshell-driver-vm/vm/control_test.go`, `quarshell-driver-vm/vm/state_test.go`, `quarshell-driver-vm/vm/lifecycle_test.go`, `quarshell-driver-vm/vmguest/agent_linux_test.go`, `quarshell-runtime/internal/app/supervisor/run_test_unix_test.go`, `quarshell-driver/tests/e2e`, `quarshell-gateway/tests/e2e`, `rpc_main_process_test.go`, `task vm:live-darwin` | Negotiation обновления протокола и versioned service/worker control contracts; live outage Gateway/daemon и foreground attach. Gateway VM recovery открыт. |
| Exec, attach и TTY | VM предоставляет отдельный full-duplex SPI extension `ExecTerminal`. Аутентифицированный guest PTY передаёт stdin/stdout, применяет resize окна, доставляет terminal control characters вроде Ctrl-C и возвращает нормализованный exit status. VM-сессии получают постоянный ID и могут подключиться к тому же гостевому процессу после обрыва control stream, а detached процесс можно завершить по session ID; недоставленный вывод хранится в ограниченном буфере. Gateway сохраняет VM session ID вместе с durable hash запроса; повтор того же TTY `request_id` подключается к прежнему процессу без повторного запуска и resume-ит вывод с последнего сохранённого cursor. Canonical main-process AttachTTY принимает стабильный `reader_id` и хранит отдельный replay cursor для каждого reader в общем lifecycle store; SDK предоставляет `AttachMainProcessWithReaderID` для повторного подключения с тем же ID. Cursor обновляется после отправки события, поэтому при разрыве в этом окне допустим повтор последнего фрейма. Канонический VM workload с `TTY=true` сохраняет main-process PTY и после пересоздания Driver подключается к тому же PID; local HVF acceptance проверяет ввод/вывод и resize 101×41. Gateway передаёт `AwaitMainProcessAttachment` через durable create state; HVF acceptance подтверждает, что короткий TTY workload удерживает вывод и exit до закрытия позднего attachment. | `quarshell-driver-vm/vm/terminal.go`, `quarshell-driver-vm/vmguest/retained_terminal.go`, `quarshell-driver-vm/vm/live_acceptance_test.go::assertLiveMainProcessTTY`, `quarshell-driver-vm/vm/live_acceptance_test.go::TestLiveMicroVMLifecycle`, `quarshell-gateway/internal/transport/applicationgrpc/sandbox.go::CreateSandbox`, `quarshell-cli/internal/service/sandbox_gateway_test.go::TestRemoteSandboxCreateForegroundAttachesToCanonicalProcess`, `quarshell-gateway/internal/transport/applicationgrpc/main_process_test.go::TestAttachTTYStreamsCanonicalMainProcessAndExit`; `task vm:live-darwin` | Full host-supervisor process bootstrap and live interactive exec on Docker/Podman hosts remain to be checked. |
| Logs, cursor и watch | Gateway log и event streams проверяются тестами. | `rpc_logs_test.go`, `rpc_watch_test.go` | Loss-aware resume, cursor expiry и reconnect parity с fixtures v0.1.4. |
| Copy и upload backpressure | Driver/SDK copy paths имеют тесты; rootfs staging использует v0.1.4 поле workspace scope. | Driver transfer tests; `rootfs_tar_test.go`; `rpc_rootfs_test.go` | Backpressure больших uploads, отмена и ограничение памяти на live engines. |
| Service forward и SSH | Gateway services и relay имеют тесты; public SSH sessions ограничены по времени и привязаны к sandbox. SSH/TCP/peer relays и supervisor exit/finalize проверяют общий durable owner epoch; после handoff активный поток закрывается в течение 250 мс. | `rpc_services_test.go`, `rpc_ssh_test.go`, `tests/e2e/engine_connect_test.go`, `runtime_relay_test.go::TestRelayStreamRejectsPendingChannelFromStaleGatewayOwner`, `runtime_relay_test.go::TestActiveSupervisorRelayIsClosedAfterDurableOwnerHandoff`, `supervisor_lease_test.go::TestSupervisorLeaseRemainsValidUntilNonzeroMainExitIsFinalized` | Host-supervisor process bootstrap из upstream и полная attach/session replay семантика; smoke опубликованного binary и проверка после daemon restart на каждом backend. |
| Provider CRUD и labels | Provider labels сохраняются, возвращаются и фильтруются до pagination. Selector validation соответствует синтаксису equality selectors. | `rpc_providers_test.go`, `store/clone_test.go` | Привязка pagination token к запросу и полные imported-provider fixtures открыты. |
| Provider profiles и refresh | Profile parsing, credential refresh, external drivers, token exchange и redaction имеют local tests. | `rpc_profiles_test.go`, `rpc_credentials_test.go`, `driver_registry_test.go` | Значения defaults и runtime effects каждого profile ещё сверяются. |
| Operator credential export | Handler v0.1.4 требует verified mTLS peer с certificate OU `operator` и отклоняет bearer auth. | `provider_credentials_export_test.go`, `grpc_auth_test.go` | Развёртывание с operator-сертификатами, parity refresh/lifetime и аудит. |
| Endpoint-bound credentials | `GetSandboxProviderEnvironment` возвращает `CREDENTIALS_WITHHELD`, пока не формируются endpoint bindings. Lint возвращает `supports_stable_placeholder=false`. | `rpc_credentials_test.go`, `rpc_profiles_test.go` | Одновременно реализовать policy-bound bindings, credential identity, stable handles, refresh и установку в supervisor. Это release blocker. |
| Policy и egress | Policy validation, updates, активные proxy decisions и credential placeholder rewriting имеют local tests. | `rpc_policy_*_test.go`, `quarshell-proxy/proxy/*_test.go` | Live тест MicroVM перезагружает revision 2, получает broker acknowledgement и проверяет allow/deny decisions. Для Docker/Podman открыты target-host dual-stack, Gateway status-report integration, Linux KVM и Podman 6 route gates. |
| Workspace, RBAC и quotas | Workspace scoping и authorization paths проверяются тестами gateway. | `rpc_workspaces_test.go`, `workspace_quota_test.go` | Полный auth-method и pagination corpus закреплённых CLI/SDK. |
| CLI и SDK parity | Go SDK methods и command dispatch собираются; есть отдельная проверка standalone consumers. Gateway MicroVM exec использует закреплённый login shell по умолчанию и `-c` для `--no-login-shell`; guest tests проверяют quoting/profile, live lane на macOS ARM64 проверяет поведение Alpine `.profile`. | `quarshell-sdk/tests/consumer`; CLI module tests; `quarshell-driver-vm/vmguest/agent_linux_test.go`; `task vm:live-darwin` | Leaf commands, flags, output, завершение процесса по timeout, Python client и published dependency resolution. |

## Backend gates

| Capability | Docker | Podman | MicroVM |
|---|---|---|---|
| Create, inspect, start, stop, delete | Docker-in-Docker E2E локально прошёл; матрица поддерживаемых host остаётся открытой | Lifecycle E2E на disposable Podman 5.8.7 прошёл; нужен rootful/rootless target-host gate | Driver-level live acceptance на macOS ARM64 прошёл, включая restart; Gateway create остаётся fail-closed до готовности host supervisor/isolation |
| CPU и память | E2E admission ресурсов прошёл; нужны проверки лимитов на целевых daemon | Поведение ресурсов зависит от host и user namespace | Live guest на macOS ARM64 видит 2 vCPU и 981 MiB при конфигурации 1 GiB; PID limit пока не поддерживается |
| Workspace persistence и ownership | E2E persistence и daemon recovery прошли; ownership и полная матрица daemon открыты | Userns/recovery случаи Podman 5.8.7 прошли с документированными capability skips; ownership на target Linux открыта | Driver live test проверяет persistent ext4 overlay. Gateway теперь выбирает guest-overlay workspace без host path; Gateway-to-VM persistence и ownership acceptance остаются открытыми. Host workspace mounts не входят в VM модель OpenShell v0.1.4 |
| Network isolation и policy broker | Docker 28 DinD E2E создаёт IPv6 isolated network, проверяет адрес sandbox и подтверждает, что sandbox не может подключиться к доступному host listener на conventional IPv6 gateway address. На одном macOS ARM64 Docker Desktop host dual-stack control sandbox подключается к host listeners по `host.docker.internal` (IPv4) и ULA IPv6 адресу хоста; proxy-isolated sandbox не достигает ни одного listener. Driver повторно сверяет identity каталога непосредственно перед Engine create; regression test подтверждает отказ до создания workload, если каталог заменили во время подготовки. Атомарная фиксация пути на стороне daemon и более широкая host matrix остаются открытыми: Engine API принимает путь, а не directory handle. | Podman 5.8.7 не поддерживает нужные blackhole route/host-gateway isolation; Podman 6 target gate открыт | Live probes на macOS ARM64 блокируют host IPv4/IPv6 loopback и подключение к unmapped host-vsock port 5504; создание raw ICMP socket отклоняется с EPERM. Reload/ack revision 2 меняет broker decision; убийство broker останавливает VM и позволяет durable recovery. Gateway acknowledgement integration, DNS и Linux KVM acceptance остаются открытыми |
| Interactive process и transfer | Package tests есть; live TTY, signals и large-file открыты | Package tests есть; live PTY, log driver и reconnect открыты | Live driver на macOS ARM64 проверяет PTY stdin/output, resize 80×24→101×41, Ctrl-C и exit status; Gateway→VM и Linux KVM остаются открыты |
| GPU / CDI / PID limits | Поддерживаются при наличии возможностей daemon; нужно evidence на целевом железе | CDI/PID support зависит от host; иначе capability должна отклоняться явно | Явно не поддерживаются до реализации и target-host evidence |
| Recovery и cleanup | Есть Gateway/daemon recovery suite; повторить на release runner | Есть rootless/rootful suite; нужны lanes рестарта service и API | Driver live test на macOS ARM64 проверяет новый экземпляр драйвера, stop/start после потери broker, перезапуск worker group после crash с прежним rootfs/overlay и cleanup; Gateway restart recovery и orphan reaper остаются открытыми |
| Release artifacts | Нужны build и smoke опубликованного binary | Нужны build и smoke опубликованного binary | Package build Linux KVM и macOS ARM64; live VM acceptance обязательна |

`ValidateGatewayRuntime` для MicroVM остаётся fail-closed. На macOS ARM64 прошёл
driver-level `TestLiveMicroVMLifecycle`: libkrun/HVF, гостевой Linux, default-deny
broker, exec/copy, supervisor readiness, подтверждённый `on-failure` restart,
SSH relay, новый экземпляр драйвера, stop/start и cleanup. Это не проверяет
Gateway orchestration. Build или unit
test без live hypervisor не считаются acceptance. MicroVM production требует
Gateway-to-workload проверки на Linux KVM x86_64/aarch64 и полного Gateway
acceptance на macOS ARM64.

## Последний локальный запуск: 2026-10-16

Последний `task check` прошёл после синхронизации `tools/modules.lock` и
workflow refs с опубликованными core v0.3.0 и proto v0.2.0. Проверка включает
workspace/image pins, форматирование, lint, vet/govulncheck и все Go tests.
Также прошли `task compat:inventory` (13 из 18 open/blocked), `task docs:strict` и
`go test -race ./internal/mounts ./internal/docker`. Также прошли
`GOWORK=off task release:standalone` и `task release:artifacts`.
`task e2e:docker-desktop` прошёл workspace и dual-stack host-boundary lanes на
macOS ARM64. `task e2e:docker` прошёл в изолированной Docker-in-Docker lane:
Gateway-to-
supervisor relay lifecycle, create/start/inspect/exec/stop/delete и proxy
allow/deny. `task e2e:podman` прошёл на disposable Podman 5.8.7 для
поддерживаемых rootful/rootless user-namespace и recovery случаев; rootful
keep-id пропущен из-за отсутствия mqueue у nested crun, rootful no-map доступен
только в rootless, rootless private maps не поддерживаются, Gateway/proxy lanes
пропущены из-за отсутствия blackhole routes в Podman 5. Podman 6.1.3 в nested Docker Desktop
упал в обязательной rootful user-namespace lane: вложенному runner недоступны
cgroup controls, а для режима `auto` отсутствовал subordinate ID range; до
proxy tests выполнение не дошло. На Podman 6.1.3 в том же nested окружении
прошли lifecycle, service recovery, pull policy, digest pinning и rootless
service recovery; эти проверки не заменяют полную user-namespace и target-host
acceptance. CI смерженного driver v0.3.0 также прошёл rootful Podman 6.1.3
тесты lifecycle, service recovery, pull policy и digest pinning, а для rootless
Podman — lifecycle, service recovery и user-namespace matrix `auto`/`host`/
`keep-id`/`no-map` на Ubuntu runners. Эти lanes не запускают proxy-backed
host-gateway и Gateway relay probes, поэтому Podman 6 isolation acceptance
остаётся открытым.
Driver PR #19, смерженный в `34060264b5627a0aac4b83d3aa4bb76f1eee9119`,
затем прошёл Podman 6.1.4 rootful и rootless proxy-isolation CI во вложенных
service containers: прямой доступ через host-gateway блокируется, proxy
allow/deny, TCP/UDP probes и отказ при потере broker проверены. Gateway relay
и native host здесь не проверялись; nested cgroup также исключает ID-map start
и acceptance лимитов ресурсов. Linux host проверка отложена по запросу.
Новая Podman 6.1.4 AppleHVF machine завершила
guest boot, но host SSH/API forwarding через `gvproxy` оказался недоступен;
существующая machine всё ещё падает в Ignition при добавлении `core`. Это не
target-host acceptance для Podman 6; нужен отдельный Linux runner. Driver
v0.3.0 опубликован после merge PR #16; CI прошёл Linux/macOS/Windows build и
test, lint/format, Linux VM artifact и rootful/rootless Podman 6.1.3 lanes.
Gateway теперь разрешает driver v0.3.0 и proto v0.2.0 из опубликованных tags;
его standalone `GOWORK=off` test проходит. Полный `task release:published`
всё ещё падает только на CLI: он закреплён на runtime v0.2.1 и SDK v0.2.0
без актуальных heartbeat/log и provider/service API. Опубликованы core v0.3.0,
proto v0.2.0, providers v0.1.8, proxy v0.3.0, runtime v0.3.0 и driver v0.3.0;
release PR SDK v0.3.0 смержен и ждёт tag после Gateway release. На macOS ARM64
прошёл driver-level
MicroVM lifecycle test через libkrun/HVF: exec/copy, deny policy, supervisor,
SSH, recovery драйвера и stop/start. Acceptance также аварийно завершает всю
worker process group, ждёт фиксации `stopped` в durable state, восстанавливает
workload spec через новый экземпляр драйвера и запускает VM повторно с
сохранёнными rootfs и overlay. Проверка наблюдает второй process instance
supervisor после exit code 7 при `on-failure`. Gateway-level MicroVM и
Linux KVM acceptance остаются открытыми. Linux worker namespace test binary
собирается для amd64 и arm64, но локальный Docker runner блокирует создание user
namespace с `EPERM`; это не Linux host acceptance. Gateway продолжает fail-closed
отклонять этот runtime.

Последний live запуск MicroVM на macOS ARM64 также проверяет protocol v1,
готовность supervisor до подтверждения старта workload, retry-safe ротацию
control token через vsock и recovery после падения worker group. Guest probes
блокируют host IPv4/IPv6 loopback и подключение к unmapped host-vsock port
5504; raw ICMP probe на macOS ARM64 получает EPERM при создании socket. Guest
видит 2 vCPU и 981 MiB при конфигурации памяти 1 GiB. Live broker перезагружает
policy revision 2 и получает ack через тестовый harness: allowlisted endpoint
проходит policy evaluation, другой endpoint по-прежнему запрещён. Gateway
status-report RPC эта проверка не вызывает. Принудительная остановка live broker
завершает worker group, сохраняет состояние `stopped` и позволяет перезапустить
VM после восстановления workload spec. Workspace host mounts, отдельный host UID,
Gateway status-report integration, Gateway recovery и Linux KVM acceptance
остаются открытыми. Эти driver-level проверки не включают MicroVM runtime в Gateway.
Отдельный subprocess test закрывает parent-liveness pipe и проверяет, что worker
завершает свою process group; обычный и image-prep worker получают такой pipe от
Gateway, чтобы не оставлять orphan VM после аварийного завершения Gateway.

Та же live lane проверяет full-duplex guest PTY: исходный размер 80×24,
resize до 101×41, stdin и Ctrl-C для команды с trap; результат — exit code 42.
Одноразовый TTY exec также получает OCI image working directory и environment.
Gateway interactive relay покрыт unit tests, но live Gateway→VM проверка не
засчитывается, пока создание VM в Gateway остаётся fail-closed.

Та же macOS ARM64/HVF lane импортирует Alpine rootfs через локальный OCI registry
с Basic Auth и изолированным Docker config. Тестовый Docker credential helper
передаёт credentials production keychain, которая аутентифицирует pull. Тест
запускает workload с image `Entrypoint`/`Cmd`, `Env`,
`WorkingDir` и `User`, проверяет внутри VM `/opt/app`, UID 1000 и image env, а
durable state — на отсутствие тестового credential и image env. После переноса
registry tag workload восстанавливается по исходному manifest digest.
Другие credential helpers и варианты TLS/auth на целевых registry, OCI pull на
Linux KVM и Gateway crash recovery остаются открытыми.

## Порядок release-тестов

План проверки от 2026-10-18 переводит общий E2E на образ Podman 6.1.4 и
включает userns matrix в rootful CI. Strict matrix разрешает только
rootful `no-map` и rootless `private` как документированные skips. Новые
workflow ещё не прошли после этого изменения; исторический результат на
Podman 5.8.7 и предшествующие ошибки вложенного Podman 6 остаются отдельными
evidence и не подтверждают текущую конфигурацию.

1. `task workspace:check`, `task check`, race/vet/vulnerability gates.
2. `task compat:pin-check`, `task compat:proto`, `task compat:inventory`.
3. `task e2e:docker` на поддерживаемой матрице Docker hosts.
4. `task e2e:podman` на отдельных rootful и rootless Podman 6 runners; proxy,
   Gateway и все обязательные userns cases должны пройти, кроме двух указанных
   mode-specific skips.
5. MicroVM Gateway-to-workload и security tests на Linux KVM x86_64/aarch64 и
   macOS ARM64; недоступный hypervisor не считается успешной проверкой.
   `task vm:live-darwin` запускает driver-level libkrun/HVF acceptance на macOS
   ARM64; `quarshell-driver/scripts/run-live-kvm-acceptance.sh` запускает Linux lane.
6. Standalone Go/Python consumers, published artifact smoke, strict docs build,
   SBOM и signature checks.

Текущий [реестр совместимости](https://github.com/KLORQ/sandbox.dev/blob/main/docs/ru/reference/remaining-migration-work.md)
содержит известные пробелы, но не доказывает, что внесено каждое исходное
поведение. Открытые строки блокируют релиз, пока к ним не приложено test evidence.
