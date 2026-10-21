# Quarshell — актуальный остаток

Дата: 2026-10-22

## Приоритет текущей работы

По запросу владельца репозитория сначала закрывается полный перенос VM-функционала из закреплённого OpenShell v0.1.4. Источник объёма работ — `quarshell-driver-vm/vm/testdata/openshell-v0.1.4-parity.json` и закреплённый checkout `tools/upstream/openshell-v0.1.4`; статус G04 сам по себе не означает полный VM parity. Остальные M-пункты не вытесняют этот приоритет до разбора всех VM parity rows. На 2026-10-22 inventory: 2 implemented, 22 partial, 2 blocked, 2 unsupported, 2 open.

Статусы: `[x]` готово, `[~]` частично, `[ ]` не начато/заблокировано.

## Что уже зелёное

- Policy: conformance 71/71, differential 71/71, parser/validation/evaluator.
- Gateway: lifecycle stores (memory/Postgres/SQLite), use cases, HTTP API, gRPC surface для workspace/sandbox/policy chunks/templates/settings/providers/services.
- Legacy `httpapi`, `cmd/cautem-gateway` и stale duplicate worktrees удалены.
- Docker: live create/start/stop/delete, resources/GPU, rootless config, egress pass-through, PTY, logs, daemon reconcile.
- macOS CI на GitHub-hosted `macos-14`: build-test PASS ([final snapshot](https://github.com/KLORQ/quarshell/actions/runs/39684411702)).
- Linux CI на GitHub-hosted `ubuntu-24.04`: workspace/DAG, Linux isolation build, полный `task test`, conformance 71/71 PASS ([final snapshot](https://github.com/KLORQ/quarshell/actions/runs/39684405826)).
- Docker и полный Podman acceptance на GitHub-hosted Linux PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39679621940)): rootful/rootless lifecycle, userns/binds, map ownership, proxy/network probes и rootless recovery. Rootful `no-map` пропущен ожидаемо как rootless-only; rootless `private` пропущен как неподдерживаемый explicit map mode.
- Release dry-run и GitHub release workflow: cross-build PASS.
- Helm chart: `helm lint`/`helm template` PASS.
- SDK/CLI tests, TS bindings build PASS.
- Evidence/DAG/workspace gates зелёные; standalone 20/20.

## M5 — gateway

- [~] Полный 83-RPC parity.
  - Готово: Health/Info, workspace, sandbox, bounded one-shot `ExecSandbox` для Docker/VM с timeout/stdin/env/output/exit; interactive TTY `ExecSandboxInteractive` для Docker/VM с stdin, half-close, resize, streamed output и exit; policy chunks/revision listing/status, templates, settings, provider definitions, sandbox provider listing, services.
  - В работе: HTTP token issue/refresh привязаны к durable runtime launch generation, отдельному от API `Generation`; каждый Gateway Start транзакционно увеличивает runtime generation и credential epoch, создаёт Gateway-owned session UUID, а reconcile переиспользует сохранённую launch identity. Gateway передаёт парные Gateway/Sandbox bearers, expiry, OpenShell runtime generation (`g` + 16 hex digits), session UUID, rotation, Gateway ID и verification keys через `RuntimeCredentialStarter`. VM доставляет verification-key bundle в root-owned guest control directory по аутентифицированному vsock до workload start. Гостевой supervisor проверяет права, JSON schema, уникальность key IDs и Ed25519 PEM, затем до старта workload проверяет Sandbox JWT подпись, JOSE-профиль, issuer/audience, sandbox ID, generation, auth epoch, component и временные claims. Токен, Gateway ID и key-file path удаляются из env workload. Session credentials подписываются Ed25519/EdDSA с OpenShell JOSE types и exact claims set; workspace не сериализуется, sandbox ID глобально уникален. VM использует те же generation/session claims в mTLS identity и mode-0600 `SupervisorAuthBundle` upstream JSON schema; bundle удаляется вместе с broker. Добавлены generation-scoped sandbox auth interceptors, `ConnectSupervisor`, `RelayStream` и loopback relay validation. Epoch подписывается в bearer и проверяется на refresh/control/relay. Gateway bearer обслуживает Gateway API, Sandbox bearer передаётся в host proxy relay для `ConnectSupervisor`/`RelayStream`; refresh обновляет оба bearer в relay, proxy server и Gateway SDK HTTP/gRPC clients. Проверка JWT пока выполняется только при старте supervisor, последующая ротация/replay semantics не реализованы. Authenticated host backend lifecycle отсутствует. Runtime validator остаётся fail-closed до end-to-end host/guest isolation handshake; bootstrap `IssueSandboxToken` остаётся fail-closed.
  - Не реализовано функционально: provider attach/detach/readiness, SSH, peer relay/report, logs/watch, supervisor/process-exit, global policy history и часть policy analysis/config RPC. Durable `request_id` admission для обоих exec RPC пока отклоняется fail-closed; interactive exec без TTY также не поддержан.
  - Оставшиеся runtime methods fail-closed `Unimplemented`/501. gRPC unary/stream interceptors проверяют общий `QUARSHELL_TOKEN`, а `GetCurrentUser` выдаёт статическую identity `gateway-client`; OIDC/per-user/sandbox claims, scope authorization и TLS остаются непокрыты, поэтому gRPC следует держать на loopback или доверенной private network.
- [~] Runtime observed-activation wiring.
- [~] Multi-gateway: shared lease store и cross-process peer RPC.
- [~] Добавлен gRPC status fixture на все 83 public RPC: status проверяется через реальные generated handlers, новые методы без fixture падают; functional fixtures с валидными use cases для каждого RPC ещё нужны.

## M6 — runtime backends

- [~] **M6a Docker/Podman**
  - Готово: Docker lifecycle, resources, rootless config, main-process and exec PTY, logs, reconcile; GitHub-hosted Linux Docker и rootful/rootless Podman acceptance PASS, включая userns/binds.
  - Egress доказан в ограниченном hosted профиле: allowed upstream CONNECT, deny TCP/UDP, прямой host-gateway bypass (Docker IPv4/IPv6, Podman rootful/rootless и proxy-down) закрыт; см. `plans/evidence/runs/hosted-linux-acceptance-2026-10-22.md`.
  - Добавлен filtered Engine `PlatformEventStream` для Docker/Podman с отказом от сырых actor attributes; mapping/cancel tests PASS. GitHub run `39684405826` подтвердил live Docker `exec_create` и Podman `exec` delivery.
  - Осталось: полный backend parity и host-level socket/DNS/UDS bypass.
- [~] **M6b Kubernetes**
  - Готово: adapter, `--runtime kubernetes`, gateway-managed workspace, config plumbing.
  - Добавлен Kubernetes `PlatformEventStream` поверх Agent Sandbox LIST/WATCH с reconnect/relist; события привязаны к immutable runtime UID, проверяется gateway ownership, произвольные CR labels/annotations/condition messages не передаются. Driver tests + race tests и hosted Linux/macOS acceptance PASS; live cluster lane ещё нужен.
  - Осталось: RBAC/namespace, NetworkPolicy enforcement, GPU/CDI, logs/exec, persistent workspace verification, cleanup after API outage, live cluster.
- [~] **M6c VM (KVM/HVF)**
  - На локальном Apple Silicon Mac прошёл реальный HVF `TestLiveMicroVMLifecycle` (`task vm:live-darwin`), включая boot/readiness, policy egress/revision ACK, ownership, recovery, registry pull, worker/broker loss и shutdown; подробности: `plans/evidence/runs/vm-hvf-local-2026-10-22.md`.
  - VM backend, worker/guest-agent binaries, guest agent, OCI/rootfs, PTY/recovery и parity inventory вынесены в отдельный модуль `quarshell-driver-vm`, подключённый в Gateway composition root; это сохраняет однонаправленную зависимость VM → driver. После переноса повторно прошёл реальный macOS ARM64/HVF lifecycle acceptance; функциональный parity остаётся неполным.
  - Gateway sandbox create передаёт network configuration backend, а Start/Stop вызывают backend lifecycle после фиксации durable intent; startup reconcile повторно запускает sandbox с persisted `running` intent, если runtime потерял процесс. `go test ./...` прошёл в VM, driver и Gateway modules; parity check проходит.
  - Требование переноса — вся VM-функциональность OpenShell v0.1.4: 29 upstream parity features плюс отдельная supplemental-строка host-supervisor bootstrap gap. Всего 30 entries: 2 implemented, 22 partial, 2 blocked, 2 unsupported, 2 open; `task vm:parity-check` проверяет pinned source anchors и наличие evidence files/symbols.
  - При запуске VM атомарно публикуется и повторно проверяется private isolation descriptor, привязанный к sandbox/session/generation, runtime/rootfs digest и mTLS credential files. Live HVF acceptance подтверждает descriptor из production lifecycle, сохранение generation при worker adoption и rotation generation при явном Stop/Start.
  - Gateway VM adapter выбирает persistent guest-overlay, передаёт настроенные host/guest runtime assets и proxy spec в Create/restore и вызывает runtime contract validator до Create/Start. Start/reconcile передают generation-bound token через transient `RuntimeTokenStarter`; VM помещает его в private broker credential file на время процесса и удаляет при завершении. Validator остаётся fail-closed, пока не подтверждён полноценный end-to-end control/isolation handshake.
  - Контрактная сверка с upstream `spawn_host_supervisor` выявила недостающую роль: host supervisor должен получить private backend descriptor/auth bundle, admitted backend, Gateway endpoint и resolved workload identity, затем выполнить attach → confirm → start. VM передаёт private descriptor path host proxy и формирует mode-0600 bundle в формате upstream `SupervisorAuthBundle`: оба bearers и expiry, UUID session, runtime generation, session rotation и auth epoch; proxy проверяет descriptor/bundle/admission bindings, использует Gateway bearer для Gateway API, а Sandbox bearer для supervisor relay `ConnectSupervisor`/`RelayStream`. `RuntimeCredentialStarter` передаёт credential pair в VM. Initial session rotation 1 совпадает с pinned OpenShell v0.1.4; последующая ротация и replay/session refresh semantics не реализованы. `IssueSandboxToken` остаётся fail-closed. Двухсторонний guest start gate теперь использует унаследованный одноразовый pipe и подтверждается только после readiness/non-root identity. Registry/backend attach и полная enforcement evidence ещё отсутствуют; `quarshell-core/isolation` не портирует upstream TCP/DNS mediation и running-boundary process/exec/loopback APIs.
  - Gateway control plane проверяет generation-bound bearer в refresh/control/relay methods; supervisor hello сверяется с sandbox record/generation, требует положительный `connection_epoch`, а RelayStream принимает только loopback TCP target. Перенесены epoch ownership и 45-секундный supervisor owner lease с heartbeat renewal; lease пока process-local, без upstream shared persistence/HA и SessionRedirect. JSON schema auth bundle и JWT wire profile совпадают с upstream; Sandbox JWT проверяется ключом в гостевом supervisor при запуске, но live HVF acceptance пока не проходит через Gateway paired-credential flow. Остаются session rotation/replay, isolation backend lifecycle и Gateway-to-VM live acceptance; G04 остаётся открытым. Локальный HVF retest подтвердил VM driver lifecycle, но не G04: `plans/evidence/runs/vm-hvf-local-2026-10-22.md`.
  - Hosted platform workflow PASS на предшествующем VM snapshot `a519ad2` ([run](https://github.com/KLORQ/quarshell/actions/runs/39756514346)): VM package tests прошли на Linux и macOS; Windows cross-build и совместимые package tests PASS. На текущем snapshot `590776d` прошли `task test` по всем Go workspace modules, Linux ARM64 и Windows supervisor cross-compiles, parity/docs gates, локальный full-history Gitleaks scan и повторный Apple Silicon/HVF driver lifecycle. GitHub не запустил secret scan ([run](https://github.com/KLORQ/quarshell/actions/runs/39762004056)) и platform jobs ([run](https://github.com/KLORQ/quarshell/actions/runs/39760554090)); все jobs завершились до шагов из-за failed payments/spending-limit у GitHub account. Live KVM/HVF hosted gates на последнем успешном platform run пропущены из-за отсутствия гипервизора; Windows runner не имеет требуемых Windows 11 Insider/MXC runtime. Подпись/notarization требуют Apple certificate secrets; GPU/VFIO требуют совместимого физического устройства.
- [~] **M6d Windows MXC**
  - `quarshell-driver-mxc` пока skeleton; GitHub-hosted Windows build общего driver + VM и совместимые package tests PASS в [platform run](https://github.com/KLORQ/quarshell/actions/runs/39756514346). Runner не имеет требуемой MXC среды; `wxc-exec.exe` и `IsoSessionApp.dll` отсутствуют.
  - MXC implementation и live contract acceptance не готовы; штатный Windows runner не считается MXC host без требуемых runtime компонентов и Insider build.
- [ ] G04: полный VM lifecycle.

## M7 — clients/tooling

- [~] Go SDK: tests зелёные; подтвердить полный typed errors/cancel/auth refresh/pagination/streaming.
- [~] CLI: команды/storage/TUI tests зелёные; сверить полное дерево с upstream inventory.
- [~] TUI workflows: базовые есть; сверить с upstream inventory.
- [~] Python/TS bindings: Python 23/23 tests и wheel build PASS (`grpcio 1.84.0`); TS build PASS; real gateway curated scenarios pending.
- [~] Go prover: bounded/risk/Z3 plumbing есть; full symbolic model pending.
- [~] EN/RU Gateway quickstarts приведены к проверенному Go/Compose запуску; в них описаны общий статический token для HTTP/gRPC, public Health и отсутствие TLS/per-user gRPC identity. Local и hosted strict build PASS ([latest Linux run](https://github.com/KLORQ/quarshell/actions/runs/39704723518)). Остальные upstream примеры и public agent skills (сейчас только policy-advisor) требуют parity review.

## M8 — release/cleanup

- [~] Удаление старого кода: `httpapi`/legacy command/stale copies удалены; старые runtime/driver packages ещё используются или не разобраны.
- [x] Защита локальных секретов: `.gitignore` покрывает env-файлы, private keys, SSH/GPG, package-manager credentials, cloud credentials и service-account files; Gitleaks CLI просканировал всю сведённую историю без совпадений ([latest source scan](https://github.com/KLORQ/quarshell/actions/runs/39703655669)).
- [!] GitHub Secret Scanning/push protection выключены у private repo; для private organization repository GitHub указывает Secret Protection entitlement. Перед включением проверить план KLORQ, чтобы не включить платную функцию без решения владельца.
- [x] Удалён tracked `opencode.jsonc` с личными absolute paths; migration review paths приведены к repo-relative или pinned-upstream ссылкам.
- [x] Очистка generated output: `task clean:generated` удаляет также Rust `target` directories; всего освобождено около 3.0 GiB, pinned source checkout сохранён.
- [x] История сведена к одному root snapshot commit на remote и локально; backup refs и reflogs удалены, `git gc --prune=now` выполнен.
- [~] Latest local VM migration: `task test` PASS across all Go workspace modules, parity inventory PASS (30 entries), Linux/Windows supervisor cross-compiles PASS, full-history Gitleaks PASS locally (one root commit, 18.21 MB), and repeated Apple Silicon/HVF driver lifecycle PASS. On current pushed snapshot `66b1e6d`, GitHub secret scan ([run](https://github.com/KLORQ/quarshell/actions/runs/39762975463)) and all manually dispatched platform jobs ([run](https://github.com/KLORQ/quarshell/actions/runs/39763010550)) stopped before steps because the account has failed payments or an exceeded spending limit. The last successful hosted VM platform run is on code snapshot `a519ad2` ([run](https://github.com/KLORQ/quarshell/actions/runs/39756514346)). Live KVM, Gateway G04, signing/notarization and remaining VM parity are incomplete.

- [~] Follow-up on `1619e99`: guest JWT verifier now rejects `exp: null` (Go otherwise decodes it as numeric zero, which is the valid OpenShell non-expiring form) while preserving `exp: 0`. Supervisor unit tests, VM parity inventory (30 entries), Linux ARM64 and Windows AMD64 supervisor cross-compiles, and local Apple Silicon/HVF lifecycle PASS. GitHub secret scan [`39763756651`](https://github.com/KLORQ/quarshell/actions/runs/39763756651) and manual platform matrix [`39763802132`](https://github.com/KLORQ/quarshell/actions/runs/39763802132) failed before any job steps (no job logs were produced). One-root history scan: one commit, 18.21 MB, no Gitleaks findings. Gateway paired-credential live acceptance and host backend attach/confirm remain unimplemented; G04 remains open.

- [~] JWT edge follow-up on `4aa3fcc`: verifier now also requires `iat` to be present as a JSON integer, rejecting omitted or null values; `exp` follows the same strict numeric rule while upstream `exp: 0` remains valid. Full workspace `task test`, Linux ARM64/Windows AMD64 cross-compiles, VM parity inventory, artifact/KVM fixture gates, and docs strict build PASS. GitHub secret-scan run [`39764231486`](https://github.com/KLORQ/quarshell/actions/runs/39764231486) failed before job steps with no logs. One-root local Gitleaks scan is clean. This closes JWT decoding edge cases only; full Gateway VM attach/confirm and G04 remain open.

- [~] macOS exec-timeout retest: `task vm:live-darwin` PASS (17.30s) after adding a live check that a timed-out `Driver.Exec` kills the guest process and prevents a delayed filesystem side effect. This extends `vm.gateway-exec-timeout` evidence but does not close the row's remaining Gateway streaming semantics or G04.

- [~] Hosted recheck on `59609c5`: manually dispatched platform matrix [`39764654224`](https://github.com/KLORQ/quarshell/actions/runs/39764654224); Linux KVM, macOS HVF, and Windows MXC jobs all failed before steps and emitted no logs. Push-triggered secret scan [`39764619947`](https://github.com/KLORQ/quarshell/actions/runs/39764619947) failed before steps as well. Local `task test`, live HVF lifecycle including exec timeout cancellation, parity, artifact/KVM fixture gates, and one-root Gitleaks passed. No hosted result is available for this VM snapshot.

- [~] Host bootstrap gap audit against pinned RFC/driver: local libkrun config creates vsock mappings, sets transparent socket interception to mask 0, and adds no network device; workload readiness/non-root identity is reported while the guest start gate is still closed. However, production code does not instantiate `quarshell-core/isolation.Registry`, and `validateGatewayRuntime()` still unconditionally returns a not-ready error after host preflight. Next implementation boundary: attach the private VM descriptor to trusted launch context, confirm measured outer-fence + broker + authenticated supervisor + non-root readiness, then release the existing gate through a production VM backend. A live Gateway→HVF acceptance with real paired credentials is still required before G04 can close. Evidence: `plans/evidence/runs/vm-hvf-local-2026-10-22.md`.
- [~] Env aliases, dual schemas, функции вне baseline.
- [~] Dockerfiles/charts/release workflow/generated clients: Gateway image build and Compose clean install (persistent SQLite, HTTP/gRPC, healthcheck) PASS locally and on GitHub-hosted Linux ([latest acceptance](https://github.com/KLORQ/quarshell/actions/runs/39703673560)); Helm, all 10 GitHub-hosted cross-builds, in-workflow manifest/SHA256/SBOM verification, and artifact upload PASS ([release CI](https://github.com/KLORQ/quarshell/actions/runs/39692425723)); registry image publish and regenerated clients remain pending.
- [~] Signatures/notarization: workflow готов, нужны Apple secrets.
- [~] Gateway-only Compose clean install прошёл локально ([evidence](evidence/runs/gateway-compose-local-2026-10-22.md)); upgrade path и published consumer clean install остаются pending.
- [ ] Published consumer без local replace: GOWORK=off gate выявил отсутствующие checksums/обновления go.mod и checksum mismatch опубликованного `quarshell-core@v0.3.0`; сначала согласовать исходники и release tags, затем повторить consumer build.
- [ ] Финальный acceptance: peer/VM/MXC/prover/GPU rows.

## Governance / contracts

- [~] Inventory: SDK operations и часть release artifacts.
- [~] Requirements synthesis в acceptance.
- [~] Release status из evidence matrix.
- [~] Strict ADR gate `core → proto` (сейчас красный).
- [~] Exact межрепозиторные source pins не заданы: `task ci:pins` пока завершается с ошибкой, потому что в `tools/modules.lock` нет опубликованных авторитетных SHA. Не подставлять теги или предположительные текущие ревизии.
- [~] TS/Python contract regeneration.
- [~] ADR для VM/MXC/prover зависимостей.

## Live gates

| Gate | Статус | Что нужно |
|---|---|---|
| Linux Landlock/seccomp/network bypass | partial | GitHub-hosted Linux build/test PASS ([latest recheck](https://github.com/KLORQ/quarshell/actions/runs/39692425422)); host-level raw-socket and DNS/UDS bypass probes pending |
| Native Linux Docker/Podman | pass | GitHub-hosted `ubuntu-24.04`: Docker and rootful/rootless Podman E2E PASS, including userns/binds, egress probes, and backend events; see hosted acceptance runs |
| Kubernetes real cluster | blocked | cluster + runtime image + guest TLS |
| KVM/HVF + signed macOS | blocked | GitHub-hosted capability probe; live KVM/HVF access and Apple signing secrets |
| Windows MXC | blocked | Windows-hosted probe; MXC Go implementation, supported Windows build, `wxc-exec.exe` and `IsoSessionApp.dll` |
| GPU/VFIO | blocked | physical hardware |
| Published consumer + clean install | blocked | registry/images |

## Ближайший порядок работ

1. Закрыть оставшийся M5 runtime parity: workload-bound tokens, interactive/durable exec, SSH, streams, peer; one-shot exec уже работает на поддерживающих его backend.
2. Добить остаток M6a: backend parity и host-level socket/DNS/UDS bypass; egress и базовая доставка backend events подтверждены.
3. M6b live cluster lane и K8s policy/RBAC.
4. M7 docs/examples, Python deps, полный SDK/CLI parity.
5. M8 images/signing/notarization/clean install.
