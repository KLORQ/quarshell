# Чек-лист выполнения плана OpenShell → Go

Baseline: `v0.1.4` / `5a7728c304b5528e954e9ae900cf21f84e1a12b1`.
Обновлено: 2026-10-21. Namespace: `github.com/KLORQ`.
План: [openshell-go-2026-10-21/PLAN.md](openshell-go-2026-10-21/PLAN.md).
Baseline-снимок: [baseline/BASELINE.md](baseline/BASELINE.md).

## Легенда

- `[ ]` не начато
- `[~]` в работе
- `[x]` готово и проверено
- `[!]` blocked внешним gate
- `[-]` вне обязательного scope

`pass` без воспроизводимой команды и артефакта не ставится. Для строк плана
допустимы `pass/fail/blocked/unverified`; `partial` не считается финальным pass.

## 1. Журнал добавлений в проект

| Дата | Что добавлено или изменено | Статус | Где |
| --- | --- | --- | --- |
| 2026-10-21 | Пакет плана и ревью принят в проект | `[x]` | `plans/openshell-go-2026-10-21/` |
| 2026-10-21 | Удалён старый `.local-ai`; исторический Go qualification snapshot перенесён в evidence | `[x]` | `plans/evidence/runs/go-source-qualification-2026-10-19.json` |
| 2026-10-21 | Добавлен индекс рабочих планов | `[x]` | `plans/README.md` |
| 2026-10-21 | Заморожен baseline и состояние рабочих деревьев | `[x]` | `plans/baseline/BASELINE.md` |
| 2026-10-21 | Добавлен живой чек-лист | `[x]` | `plans/CHECKLIST.md` |
| 2026-10-21 | Корневой README ссылается на рабочий план | `[x]` | `README.md` |
| 2026-10-21 | Инструменты больше не пишут в `.local-ai` | `[x]` | `.gitignore`, `tools/e2e/docker-desktop.sh`, `tools/e2e/README.md`, `tools/generate-openshell-inventory.py` |
| 2026-10-21 | Выход inventory-генератора перенесён в `plans/inventory/` | `[x]` | `plans/inventory/README.md` |
| 2026-10-21 | Inventory привязан к новому плану: 37 crates, 7 сервисов, 110 RPC, 29 feature rows | `[x]` | `plans/inventory/` |
| 2026-10-21 | Namespace переведён на `github.com/KLORQ`: module paths, imports, go.work, npm scope, docs URL, git remotes | `[x]` | workspace |
| 2026-10-21 | Из `go.mod` новых модулей убраны `replace`; локальные зависимости разрешает `go.work` | `[x]` | `go.work`, `tools/workspace.py` |
| 2026-10-21 | Stream/provider/release inventory: 21 stream message, 84 variant, 17 profiles, 4 platform rows | `[x]` | `plans/inventory/` |
| 2026-10-21 | Evidence matrix, validator, R/G backlog, шесть ADR | `[x]` | `plans/evidence/`, `plans/backlog/`, `plans/decisions/` |
| 2026-10-21 | M1: domain-контракты, Compute/Isolation/Credential/Extension SPI, каркас девяти модулей | `[x]` | `quarshell-core/domain`, `quarshell-spi`, новые modules |
| 2026-10-21 | M2: bounded parser, presence/unknown/null, policy corpus и differential harness | `[x]` | `quarshell-policy`, `plans/conformance/`, `tools/conformance.py` |
| 2026-10-21 | M1: composition root `quarshell-gateway` с `/healthz`, `/readyz`, `/capabilities` без старого engine | `[x]` | `quarshell-gateway/cmd/quarshell-gateway` |
| 2026-10-21 | После KLORQ rename исправлены SDK/CLI package selectors; build sweep всех 19 modules зелёный | `[x]` | `quarshell-cli`, `quarshell-sdk` |
| 2026-10-21 | Проверки: targeted tests, CLI service, 12/12 policy conformance, evidence validator, workspace check | `[x]` | см. команды ниже |
| 2026-10-21 | R01: parser-enforced `MaxEvents`; boundary tests для bytes/depth/events/nodes/keys/elements/aliases/scalar; дополнительные corpus cases | `[x]` | `quarshell-policy/budget_test.go`, `plans/conformance/corpus/` |
| 2026-10-21 | R02: presence-тесты absent/null/false/zero/empty object/empty list/unknown; null scalar остаётся null, null port отклоняется | `[x]` | `quarshell-policy/presence_test.go`; upstream differential добавлен ниже |
| 2026-10-21 | Pure evaluator подключён к conformance: expected decisions для filesystem/network/process | `[x]` | `tools/conformance.py`, `plans/conformance/corpus/eval/` |
| 2026-10-21 | R02: typed decoder отклоняет null для всех authored-полей; `policy-check` проверяет `Typed()`; null list/tool/mcp/landlock/middleware cases | `[x]` | `quarshell-policy/typed.go`, `plans/conformance/corpus/cases/` |
| 2026-10-21 | Дифференциальный gate против pinned upstream: Rust oracle, `task conformance:upstream`, 71/71 cases | `[x]` | `tools/policy-oracle/`, `tools/check-policy-differential.sh` |
| 2026-10-21 | M2: provider policy composition — reserved `_provider_` keys, collision suffix, strip, deep copy без мутации source/layers | `[x]` | `quarshell-policy/compose.go`, `compose_test.go` |
| 2026-10-21 | M1: module DAG/import-cycle gate; 20 modules, 0 cycles, strict ADR warnings на `core → proto` | `[x]` | `tools/check-module-dag.py` |
| 2026-10-21 | M2: global-replaces-sandbox precedence, derived policy hash, provider composition | `[x]` | `quarshell-policy/compose.go` |
| 2026-10-21 | M2: `FuzzParse`, memory (64 MiB) и time (2 s) budgets, `task policy:fuzz` | `[x]` | `quarshell-policy/fuzz_test.go`, `Taskfile.yml` |
| 2026-10-21 | M3: launch admission/sanitization; Linux restricted child (socketpair + SO_PEERCRED, Landlock, no_new_privs, capability/identity drop); supervisor session + bounded reporter | `[~]` | `quarshell-isolation/`, `quarshell-supervisor/` |
| 2026-10-21 | M3: Linux amd64/arm64 cross-build и test-compile; live kernel lanes blocked | `[!]` | `GOOS=linux go build ...` |
| 2026-10-21 | M3: FD injection — regular-file sources, temp duplication, target `dup3`, admission range; cross-built | `[x]` | `quarshell-isolation/linux.go` |
| 2026-10-21 | M3: restart policy (`never`/`on_failure`/`always`, bounded attempts) и session re-prepare | `[x]` | `quarshell-supervisor/session.go` |
| 2026-10-21 | M3: procfs descendant scanner spike с bounded scan и явным ограничением polling | `[~]` | `quarshell-supervisor/processes.go` |
| 2026-10-21 | M3: cgroup v2 manager + Launch integration (pids/memory/cpu limits, empty-dir cleanup) | `[x]` | `quarshell-isolation/cgroup.go`, `linux.go` |
| 2026-10-21 | M5: durable lifecycle core — атомарные entity+operation записи, idempotent execute, outbox leases/retries/DLQ, reconciler | `[x]` | `quarshell-gateway/internal/application/lifecycle/` |
| 2026-10-21 | M5: sandbox use case — идемпотентные Create/Delete, Get, outbox events, replay после restart | `[x]` | `quarshell-gateway/internal/application/sandbox/` |
| 2026-10-21 | M2: Kubernetes operator key `default_runtime_class_name` вместо legacy `runtime_class_name` | `[x]` | `quarshell-driver/internal/kubernetes/backend_config.go` |
| 2026-10-21 | M2: audit K8s/VM backend key coverage; `task compat:backends` | `[~]` | `tools/check-backend-config-fields.py` |
| 2026-10-21 | M2: K8s `image_pull_policy` + snake_case/Kubernetes pull-policy spellings | `[x]` | `quarshell-driver/internal/kubernetes/backend_config.go` |
| 2026-10-21 | M5/R03: combined watch — независимое закрытие источников, per-source errors, backpressure, cancel | `[x]` | `quarshell-gateway/internal/application/watch/` |
| 2026-10-21 | M2 closure: ADR-0007 — implemented или fail-closed reject; backend audit и corpus | `[x]` | `plans/decisions/0007-backend-config-scope.md` |
| 2026-10-21 | M3: PTY sessions, resize, signals и exit statuses в supervisor | `[x]` | `quarshell-supervisor/exec_unix.go` |
| 2026-10-21 | M3: seccomp baseline denylist в Linux child; notification broker остаётся blocked | `[~]` | `quarshell-isolation/seccomp_linux.go` |
| 2026-10-21 | M4: encrypted credential stores, broker handles, refresh/R04, placeholder rewriting | `[x]` | `quarshell-credentials/` |
| 2026-10-21 | M4: default-deny mediation + credential rewrite + create/rotate/detach scenario | `[x]` | `quarshell-proxy/internal/application/mediation/` |
| 2026-10-21 | M4: session-bound provider readiness/ACK с revision fencing | `[x]` | `quarshell-providers/provider/readiness.go` |
| 2026-10-21 | M8: local module proxy — все 20 modules проходят `GOWORK=off` build | `[x]` | `tools/standalone-build.py` |
| 2026-10-21 | M7: bounded policy containment prover с counterexample/Inconclusive | `[x]` | `quarshell-prover/containment.go` |
| 2026-10-21 | R05: generated evidence report `STATUS.md` + `--check` gate | `[x]` | `tools/generate-evidence-report.py` |
| 2026-10-21 | M5: ownership lease с fencing tokens, renewal/expiry/stale-owner rejection | `[x]` | `quarshell-gateway/internal/application/lifecycle/lease.go` |
| 2026-10-21 | M4: ordered middleware chain с auth, timeout и bounded failures | `[x]` | `quarshell-proxy/internal/application/chain/` |
| 2026-10-21 | M7: risk analysis prover для broad grants | `[x]` | `quarshell-prover/risk.go` |
| 2026-10-21 | M4: Vault KV v2 credential store с локальным шифрованием и CAS | `[x]` | `quarshell-credentials/vault_store.go` |
| 2026-10-21 | M8: local module proxy — 20/20 modules проходят standalone build | `[x]` | `tools/standalone-build.py` |
| 2026-10-21 | M4: real HTTP forward proxy + CONNECT tunnel + default deny на localhost | `[x]` | `quarshell-proxy/internal/application/httpproxy/` |
| 2026-10-21 | M4: TLS MITM для CONNECT с in-memory CA и short-lived leaf certs | `[x]` | `quarshell-proxy/internal/application/httpproxy/tls.go` |
| 2026-10-21 | M4: proxy client auth — constant-time static bearer token, 407, header stripping | `[x]` | `quarshell-proxy/internal/application/httpproxy/auth.go` |
| 2026-10-21 | M4: credential header injection с endpoint binding и stripping control headers | `[x]` | `quarshell-proxy/internal/application/httpproxy/credential_test.go` |
| 2026-10-21 | M5: workspace/member use cases, RBAC roles, idempotent create/delete, replay | `[x]` | `quarshell-gateway/internal/application/workspace/` |
| 2026-10-21 | M5: policy draft/approve/reject/history/active с validation и supersede | `[x]` | `quarshell-gateway/internal/application/policy/` |
| 2026-10-21 | M5: durable provider bindings, revision fencing, readiness ACK/Fail/Unbind | `[x]` | `quarshell-gateway/internal/application/provider/` |
| 2026-10-21 | M5: durable event logs (memory/file) с cursor, retention и lost-cursor | `[x]` | `quarshell-gateway/internal/application/eventlog/` |
| 2026-10-21 | M5: settings, templates, quotas, relay session registry; policy analysis + observed generation | `[x]` | `quarshell-gateway/internal/application/{settings,template,quota,relay,policy}/` |
| 2026-10-21 | M5: sandbox services и port forwards registries с idempotency/outbox | `[x]` | `quarshell-gateway/internal/application/endpoint/` |
| 2026-10-21 | M5: PostgreSQL lifecycle store с транзакциями, CAS, outbox claim/retry/DLQ | `[x]` | `quarshell-gateway/internal/application/lifecycle/postgres.go` |
| 2026-10-21 | M5: SQLite lifecycle store (single-node) с CAS/outbox/reopen | `[x]` | `quarshell-gateway/internal/application/lifecycle/sqlite.go` |
| 2026-10-21 | M5: peer registry с ownership lease, fencing и stale-owner rejection | `[x]` | `quarshell-gateway/internal/application/peer/` |
| 2026-10-21 | M5: HTTP transport slice над workspace/sandbox use cases с bearer auth и typed errors | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/` |
| 2026-10-21 | M5: policy SSE stream с cursor/Last-Event-ID и hot-update events | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/` |
| 2026-10-21 | M5: policy HTTP lifecycle endpoints (drafts/approve/reject/observe/active/history/analysis) | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/server.go` |
| 2026-10-21 | M5: settings/templates/quotas HTTP endpoints | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/` |
| 2026-10-21 | M5: providers/services/forwards HTTP endpoints | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/` |
| 2026-10-21 | M5: admission deadline + generation fencing в sandbox/policy | `[x]` | `quarshell-gateway/internal/application/admission/` |
| 2026-10-21 | M5: network relay с bounded exponential reconnect и eventlog | `[x]` | `quarshell-gateway/internal/application/relay/network.go` |
| 2026-10-21 | M5: peer owner/validate/peers HTTP transport с fencing | `[x]` | `quarshell-gateway/internal/transport/peerhttp/` |
| 2026-10-21 | M5: peer manager с heartbeat, takeover и fail-closed RequireLeader | `[x]` | `quarshell-gateway/internal/application/peer/manager.go` |
| 2026-10-21 | M5: secrets metadata/handles, gateway info/health и generic events HTTP | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/` |
| 2026-10-21 | M5: whoami HTTP поверх статического principal | `[x]` | `quarshell-gateway/internal/transport/applicationhttp/server.go` |
| 2026-10-21 | M5: sandbox Start/Stop с generation fence и idempotency | `[x]` | `quarshell-gateway/internal/application/sandbox/service.go` |
| 2026-10-21 | M5: provider CRUD и profiles import/lint HTTP | `[x]` | `quarshell-gateway/internal/application/provider/registry.go` |
| 2026-10-21 | M5: policy chunks edit/approve/reject/undo/clear/history HTTP | `[x]` | `quarshell-gateway/internal/application/policy/chunks.go` |
| 2026-10-21 | M5: sandbox token issue/refresh с HMAC и expiry | `[x]` | `quarshell-gateway/internal/application/token/service.go` |
| 2026-10-21 | M5: peer join/claim/heartbeat/release HTTP | `[x]` | `quarshell-gateway/internal/transport/peerhttp/server.go` |
| 2026-10-21 | M5: exec/SSH/stream и peer relay/report fail-closed 501 adapters | `[x]` | `quarshell-gateway/internal/transport/` |
| 2026-10-21 | M5: gRPC adapter с legacy fallback и новым Health | `[x]` | `quarshell-gateway/internal/transport/applicationgrpc/` |
| 2026-10-21 | M5: gRPC adapter hooks для CurrentUser/GatewayInfo | `[x]` | `quarshell-gateway/internal/transport/applicationgrpc/server.go` |
| 2026-10-21 | M5: greenfield composition root `cmd/quarshell-gateway` на lifecycle store и application HTTP | `[x]` | `quarshell-gateway/cmd/quarshell-gateway/main.go` |
| 2026-10-21 | M5: удалён legacy `httpapi`, `cmd/cautem-gateway` и их integration/e2e tests (backup `/tmp/quarshell-legacy-backup.tgz`) | `[x]` | `quarshell-gateway/` |
| 2026-10-21 | M5: greenfield gRPC server (Health/Info, остальные 82 RPC — явный Unimplemented) подключён к `cmd/quarshell-gateway` | `[x]` | `quarshell-gateway/internal/transport/applicationgrpc/server.go` |
| 2026-10-21 | M5: gRPC workspace/sandbox/policy/templates/settings/providers/services + fail-closed runtime RPC | `[x]` | `quarshell-gateway/internal/transport/applicationgrpc/` |
| 2026-10-21 | M4: Kubernetes Secrets store на client-go fake и PostgreSQL store | `[x]` | `quarshell-credentials/kubernetes_store.go`, `postgres_store.go` |
| 2026-10-21 | Credentials live lanes: Vault dev + kind cluster + Postgres прошли на macOS | `[x]` | `credentials:vault`, `credentials:kind`, `credentials:postgres` |
| 2026-10-21 | SPIRE Workload API adapter + live server/agent lane в Docker | `[x]` | `credentials:spire`, `quarshell-credentials/spiffe_source.go` |
| 2026-10-21 | Docker Desktop disk увеличен 264→320 GiB для live credential lanes | `[x]` | Docker settings |
| 2026-10-21 | M4: OAuth RFC 8693 token exchange с bounds/timeout | `[x]` | `quarshell-credentials/oauth_exchange.go` |
| 2026-10-21 | M4: SPIFFE JWT-SVID source interface + provider token grant composition | `[x]` | `quarshell-credentials/token_grant.go` |
| 2026-10-21 | M7: Z3 SMT-LIB solver + port containment + deterministic fallback | `[x]` | `quarshell-prover/solver.go`, `symbolic.go` |
| 2026-10-21 | CI: lane matrix для macOS/Linux/Windows/GPU и локальные macOS-замены | `[x]` | `plans/ci-matrix.md` |
| 2026-10-21 | M8: release dry-run — darwin/linux/windows binaries, SHA256SUMS, SBOM manifest | `[x]` | `tools/release-dry-run.sh` |
| 2026-10-21 | M3: durable local event log — JSONL append, fsync, mode 0600 | `[x]` | `quarshell-supervisor/reporter_file.go` |
| 2026-10-21 | `go.work` перегенерирован: добавлен replace для `quarshell-isolation v0.0.0` | `[x]` | `go.work` |
| 2026-10-21 | M2: MCP canonical forms, supported-version/duplicate/empty validation, mcp-only endpoint semantics; 6 corpus cases | `[x]` | `quarshell-policy/mcp.go`, `plans/conformance/corpus/` |
| 2026-10-21 | M2: full runtime policy validation, endpoint mode cases, `quarshell-policy-validate`; 71/71 parse + 71/71 validate differential | `[x]` | `quarshell-policy/validate.go`, `cmd/quarshell-policy-validate/` |
| 2026-10-21 | M2: `Snapshot` теперь отклоняет невалидную policy через `Validate()` до публикации | `[x]` | `quarshell-policy/snapshot.go` |
| 2026-10-21 | M2: typed L7 matchers (glob/any/params), REST query, GraphQL, MCP, SQL, JSON-RPC, access presets; eval corpus расширен | `[x]` | `quarshell-policy/l7_matcher.go`, `l7_match.go`, `plans/conformance/corpus/eval/` |
| 2026-10-21 | M2: validation edge cases — missing host/port, TCP DNS/IP, TLD/wildcard shape, filesystem limits, MCP defaults; 71/71 parse+validate differential | `[x]` | `quarshell-policy/validate.go`, `plans/conformance/corpus/` |
| 2026-10-21 | M2: gateway TOML schema audit (0 missing, 10 legacy), `image_preparation_timeout_seconds`, removed `builtin` provider source | `[x]` | `tools/check-gateway-config-fields.py`, `quarshell-gateway/internal/gatewayconfig/` |
| 2026-10-21 | M2: removed all legacy gateway inheritance fields; gateway audit 0 missing / 0 legacy, `--strict` gate | `[x]` | `quarshell-gateway/internal/gatewayconfig/` |
| 2026-10-21 | Проверки: race tests policy/isolation/supervisor, Linux amd64/arm64 cross-build, 71/71 conformance, 71/71 upstream differential, evidence validator, workspace check | `[x]` | см. команды ниже |
| 2026-10-22 | M8: исправлен Gateway image build context для всех 20 workspace modules, version ldflag, entrypoint, HTTP liveness/auth и HTTP/gRPC bind; default image собран и стал healthy | `[x]` | `quarshell-gateway/Dockerfile`, `quarshell-gateway/README.md`, `quarshell-gateway/cmd/quarshell-gateway/main.go`, `quarshell-gateway/internal/transport/applicationhttp/` |
| 2026-10-22 | M8/M7: Compose Gateway clean install создаёт SQLite, поднимает HTTP/gRPC и healthcheck; EN/RU quickstart исправлен под текущий Go server, `task docs:strict` PASS | `[x]` | `quarshell-gateway/compose/`, `quarshell-docs/docs/{en,ru}/get-started/gateway.md`, `plans/evidence/runs/gateway-compose-local-2026-10-22.md`, [hosted acceptance](https://github.com/KLORQ/quarshell/actions/runs/39700634881) |
| 2026-10-22 | CI: добавлен Ubuntu-hosted `task docs:strict` job; strict build и runner cleanup PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39700634881)) | `[x]` | `.github/workflows/linux.yml`, `scripts/check-docs-strict.sh` |
| 2026-10-22 | M5: закрыт unauthenticated gRPC доступ — unary/stream используют общий `QUARSHELL_TOKEN`; `GetCurrentUser`, auth/bufconn tests и status fixtures для всех 83 RPC; Linux/macOS acceptance и full-history Gitleaks PASS ([Linux](https://github.com/KLORQ/quarshell/actions/runs/39703673560), [macOS](https://github.com/KLORQ/quarshell/actions/runs/39703678330), [Gitleaks](https://github.com/KLORQ/quarshell/actions/runs/39703655669)) | `[x]` | `quarshell-gateway/internal/transport/applicationgrpc/`, `cmd/quarshell-gateway/` |
| 2026-10-22 | M5: `ListSandboxProviders` подключён к durable bindings и provider registry с workspace/sandbox check и pagination; gateway tests, Linux/macOS hosted acceptance и Gitleaks PASS ([Linux](https://github.com/KLORQ/quarshell/actions/runs/39704723518), [macOS](https://github.com/KLORQ/quarshell/actions/runs/39704728098), [Gitleaks](https://github.com/KLORQ/quarshell/actions/runs/39704702486)) | `[x]` | `quarshell-gateway/internal/transport/applicationgrpc/provider_binding.go` |
| 2026-10-22 | M5: `ListSandboxPolicies` и `GetSandboxPolicyStatus` работают по workspace/sandbox revision service: bounded scope-bound pagination, hash/status/timestamps, lookup по версии и активной observed версии; policy source не раскрывается; global history остаётся на legacy storage; gateway/full module tests, Linux/macOS hosted acceptance и Gitleaks PASS ([Linux](https://github.com/KLORQ/quarshell/actions/runs/39708214740), [macOS](https://github.com/KLORQ/quarshell/actions/runs/39708214883), [Gitleaks](https://github.com/KLORQ/quarshell/actions/runs/39708214509)) | `[~]` | `quarshell-gateway/internal/transport/applicationgrpc/policy_revisions.go` |
| 2026-10-22 | M5: unary-server-streaming `ExecSandbox` связан с backend `InteractiveExec` для Docker/VM: sandbox lookup, timeout/cancel, deterministic env conversion, stdin, TTY size, chunked stdout/stderr и exit; interaction bidi и durable request_id ещё не покрыты; gateway/driver tests PASS | `[~]` | `quarshell-gateway/internal/transport/applicationgrpc/exec.go`, `quarshell-driver/driver/driver.go` |
| 2026-10-22 | M5: bidi `ExecSandboxInteractive` связан с Docker/VM PTY sessions: start metadata, streaming stdin, stdin EOF/half-close, resize, stdout and exit; durable request_id и non-TTY interactive path остаются fail-closed; full workspace and race suites PASS | `[~]` | `quarshell-gateway/internal/transport/applicationgrpc/exec.go`, `quarshell-driver/internal/docker/exec_terminal.go`, `quarshell-driver-vm/vm/terminal.go` |
| 2026-10-22 | M6c: VM backend/worker/guest-agent выделены в самостоятельный `quarshell-driver-vm`; VM регистрируется из Gateway composition root, убирая обратную зависимость `driver → VM`; sandbox create передаёт network options в backend, Start/Stop вызывают backend, reconcile восстанавливает persisted running intent после потери runtime; workspace tests, live HVF lifecycle и parity inventory PASS. Двухсторонний guest start gate подтверждён live, но он не реализует upstream host isolation backend attach/confirm/start; upstream auth bundle и G04 остаются неполными | `[~]` | `quarshell-driver-vm`, `quarshell-gateway/internal/application/sandbox/service.go`, `quarshell-gateway/internal/application/reconcile/reconcile.go` |
| 2026-10-22 | M5/M6c: session credentials use the upstream EdDSA JOSE types and exact strict claim set, with Gateway issuer/audience, SPIFFE subject, global sandbox ID, runtime generation, auth epoch, component and UUID `jti`. Workspace remains outside the JWT and sandbox IDs are globally unique. Each Start advances runtime generation/epoch and creates a Gateway-owned session UUID; reconcile reuses the identity. VM carries generation/session through mTLS identity, descriptor and mode-0600 `SupervisorAuthBundle`. Host proxy separates Gateway API and Sandbox Protocol bearers; refresh rotates both across relay, proxy and SDK clients. Gateway ID and verification keys accompany launch credentials; VM validates Ed25519 keys and transfers them to the guest root-owned control directory before workload start. Guest supervisor validates file permissions, strict key records and Ed25519 PEM before workload start. Initial session rotation 1 matches pinned OpenShell v0.1.4. JWT verification inside the full guest boundary protocol, subsequent session rotation/replay, backend attach/confirmation and G04 remain open | `[~]` | `quarshell-gateway/internal/application/token/service.go`, `quarshell-gateway/internal/application/sandbox/service.go`, `quarshell-driver-vm/vm/lifecycle.go`, `quarshell-driver-vm/vm/network_broker_unix.go`, `quarshell-cli/internal/service/proxy_token.go`, `quarshell-runtime/relayclient/session.go`, `quarshell-sdk/go/klorq/client.go` |
| 2026-10-22 | M6b: Kubernetes backend exposes filtered Agent Sandbox platform events with runtime UID and gateway ownership checks; driver tests/race tests, hosted Linux/macOS acceptance, and Gitleaks PASS ([Linux](https://github.com/KLORQ/quarshell/actions/runs/39705921163), [macOS](https://github.com/KLORQ/quarshell/actions/runs/39705926419), [Gitleaks](https://github.com/KLORQ/quarshell/actions/runs/39705899255)); fake LIST/WATCH tests cover snapshots, updates, delete, filtering, and CR data redaction; live cluster still pending | `[~]` | `quarshell-driver/internal/kubernetes/platform_events.go` |
| 2026-10-22 | M8: `.gitignore` покрывает env, SSH/GPG, cloud/service credentials и локальные Codex/Claude/Agents/Cursor config dirs | `[x]` | `.gitignore` |

Новая строка добавляется в эту таблицу в той же задаче, где меняется продукт.

## 1.1. Команды проверки

```bash
python3 tools/workspace.py --check
python3 tools/check-module-dag.py
python3 tools/check-evidence-matrix.py
python3 tools/conformance.py
tools/check-policy-differential.sh   # or: task conformance:upstream
go test ./quarshell-core/domain ./quarshell-spi/... ./quarshell-policy/... ./quarshell-isolation/... ./quarshell-supervisor/...
task isolation:linux-build
go -C quarshell-cli test ./internal/service/...
go -C quarshell-gateway test ./internal/application/health ./internal/transport/http
```

## 2. Целевые модули: что добавляем

План задаёт 17 целевых Go modules. Текущий workspace содержит 20 Go modules.
Таблица показывает
разницу и судьбу текущих каталогов.

| Целевой module | Сейчас | Действие | Статус |
| --- | --- | --- | --- |
| `quarshell-core` | есть | Пересобрать domain IDs, generation, errors | `[~]` |
| `quarshell-policy` | нет, policy внутри `core` | Выделить parser → AST → evaluator | `[~]` |
| `quarshell-proto` | есть | Один pinned source, регенерация transport | `[~]` |
| `quarshell-spi` | нет | Добавить Compute/Isolation/Credential/extension contracts | `[~]` |
| `quarshell-telemetry` | нет | OCSF/OTEL/redaction; поглощает роль `slogx` | `[~]` |
| `quarshell-providers` | есть | Переписать под новый contract | `[ ]` |
| `quarshell-credentials` | нет | Broker, leases, refresh, DB/K8s/Vault | `[~]` |
| `quarshell-proxy` | есть | L7, middleware, credential injection | `[ ]` |
| `quarshell-isolation` | нет, часть `runtime` | Linux launcher/broker, identity, FD boundary | `[~]` |
| `quarshell-supervisor` | нет, часть `runtime` | Sessions, admission, process orchestration | `[~]` |
| `quarshell-driver` | есть | Docker/Podman/Kubernetes на новом SPI | `[ ]` |
| `quarshell-driver-vm` | есть: VM backend и runtime перенесены из `driver`; локальный macOS ARM64/HVF lifecycle PASS после переноса | Довести OpenShell VM parity: host supervisor/auth/isolation, KVM/HVF release, lifecycle gaps, GPU/VFIO | `[~]` |
| `quarshell-driver-mxc` | нет | Windows MXC | `[~]` |
| `quarshell-gateway` | есть | Transport → application → domain/ports | `[ ]` |
| `quarshell-sdk` | есть | Public Go client без server-зависимостей | `[~]` |
| `quarshell-cli` | есть | Полное дерево команд, TUI, bootstrap | `[ ]` |
| `quarshell-prover` | нет | Symbolic model, Z3 port, reports | `[~]` |

Старые каталоги:

| Каталог | Судьба | Статус |
| --- | --- | --- |
| `quarshell-runtime` | Заменяется на `isolation` + `supervisor`, затем удаляется | `[ ]` |
| `quarshell-slogx` | Роль переходит к `telemetry`; published MIT path сохранить или закрыть отдельным решением | `[ ]` |
| `quarshell-display` | Вне обязательного upstream scope | `[-]` |
| `quarshell-ui` | Вне обязательного upstream scope | `[-]` |
| `quarshell-python` | Остаётся тонким клиентом | `[ ]` |
| `quarshell-org-profile` | Проверить необходимость; upstream-аналога нет | `[ ]` |

## 3. M0 — Заморозить спецификацию и scope

- [x] Зафиксировать upstream version/SHA и сохранить артефакты ревью.
- [x] Записать состояние всех рабочих деревьев.
- [~] Построить полный inventory: crates/RPC/CLI/config/stream/providers/platform готовы; остаются SDK operations и часть release artifacts.
  - [x] 37 crates с owner и feature IDs.
  - [x] 7 сервисов / 110 RPC, включая `IsolationBoundary.Exchange` и `.Mediate`.
  - [x] 95 CLI commands и 272 argument definitions.
  - [x] 123 config types и 249 test files.
  - [x] 21 stream message и 84 message variant.
  - [x] 17 provider profiles с credentials, endpoints и refresh strategies.
  - [x] Release channels, 4 platform rows, gateway/sandbox artifacts.
- [~] Выписать требования из поведения и тестов: tests/docs/installers индексированы, synthesis в acceptance ещё не завершён.
- [x] Перенести R01–R05 и G01–G07 в backlog с acceptance.
- [x] Зафиксировать правило: нет обратной совместимости со старым Quarshell; upstream contract pinned (ADR-0006).
- [~] Генерировать release status из evidence matrix: matrix и validator есть; генерация docs/release ещё не подключена.
- [x] Создать первый differential harness: 12 policy cases, candidate против expected/oracle.
- [x] Каждая непроверенная строка помечена `unverified`, не `pass`; validator блокирует неполный pass.

**Выход M0:** `[~]` feature matrix и harness готовы; SDK operations и release docs generation остаются.

## 4. M1 — Новые контракты и skeleton модулей

- [~] Создать DAG модулей из раздела 3; supervisor/isolation и compute разделены, cycle gate автоматизирован (`task dag:check`); strict ADR gate пока красный на `core → proto`.
- [x] Утвердить domain IDs, errors, generations, capabilities, CPU/memory/GPU units, presence semantics.
- [ ] Утвердить repositories, schema, idempotency/outbox/lease и standalone build.
- [~] Сгенерировать transport contracts из pinned schemas: Go regenerated с KLORQ go_package; TS/Python regeneration pending.
- [x] Создать composition roots и сквозную операцию health/capabilities без старого engine (`cmd/quarshell-gateway`).
- [~] Определить matrix системных зависимостей: platform matrix и toolchain pin есть; VM/MXC/prover dependency ADR ещё нет.

**Выход M1:** `[~]` новые modules собираются в workspace; cycle gate есть, standalone build и strict ADR gate остаются.

## 5. M2 — Policy и конфигурация

- [x] Bounded parser: input 4 MiB, structural budgets, presence/null/unknown/duplicate/merge; boundary tests покрывают limit−1/limit/limit+1, depth, events, nodes, aliases, keys, elements, scalar; differential с pinned upstream 71/71.
- [x] Authored schema: presence, unknown fields, null rules, MCP canonical forms, full runtime validation (host/TLD/wildcard/DNS, filesystem limits, middleware, L7 semantics).
- [x] Typed config каждого backend: Docker/Podman и gateway TOML полностью; K8s/VM implemented keys typed, остальные fail-closed reject (ADR-0007); MXC behavior в M6.
- [x] Pure evaluator для filesystem/process/network/L7: filesystem/process/network + REST query, GraphQL, MCP tool/params, SQL, JSON-RPC и access presets; тело протоколов парсит proxy, evaluator получает нормализованные поля.
- [x] Provider/global/sandbox composition, hashes/revisions, immutable snapshots: provider layers, global-replaces-sandbox precedence, derived hash и Snapshot.
- [x] Corpus: policy/eval 71 cases, gateway TOML fixtures, 54-полевой driver matrix и K8s/VM key audit.
- [x] Differential/property/fuzz: pinned upstream oracle 71/71, `FuzzParse`, memory/time budgets, `task policy:fuzz`.

**Выход M2:** `[x]` parser/validation/evaluator/composition/differential/config schema готовы; backend behavior для rejected keys принадлежит M6.

## 6. M3 — Linux isolation и supervisor · высокий риск

- [x] Go launcher и supervisor: restricted child re-exec (socketpair spec, SO_PEERCRED), session state machine, bounded reporting, PTY/signals.
- [~] Capabilities, identity, no_new_privs, namespaces, Landlock, cgroup v2 limits и seccomp baseline denylist: реализованы в Linux child; seccomp notification broker pending.
- [~] Network mediation, procfs identity, FD injection, provider files: Landlock port rules, /proc start-time fence, FD allowlist/dup-install и file sources есть; host L7 mediation belongs to M4.
- [x] Main/exec/PTY/reaper/signals: reaper, exit statuses, restart policy, PTY sessions, resize и signals.
- [x] Config admission и reporting: `ValidateLaunchSpec`, `BoundedReporter`, fsync `FileReporter`; durable cross-node store belongs to M5.
- [!] Event source spike: procfs polling baseline есть; kernel event source (pidfd/proc connector/eBPF) требует Linux host lane.
- [!] Linux amd64/arm64 checks: cross-build/vet проходит; live kernel/host lanes blocked внешним gate.

**Выход M3:** `[!]` код restricted child, PTY, cgroups, Landlock и seccomp baseline готов; live kernel acceptance и seccomp notification broker блокированы внешним Linux gate.

## 7. M4 — Egress, providers, credentials, extensions

- [x] HTTP/TCP/TLS proxy и default-deny: HTTP forward proxy, CONNECT tunnel, TLS MITM с in-memory CA, body limits, client auth и credential header injection.
- [~] L7 protocol matrix: policy evaluator + credential-safe rewriting готовы; proxy body parsing/limits integration pending.
- [x] Profiles/catalog; encrypted memory/file stores; Vault KV v2, Kubernetes Secrets, PostgreSQL и SPIRE Workload API; live Vault/Postgres/kind/SPIRE lanes пройдены на macOS.
- [x] Stable handles, endpoint binding, expiry/revoke/refresh, managed files via FD, OAuth token grants и SPIRE JWT-SVID adapter; live SPIRE lane пройден.
- [x] Session-bound provider readiness и ACK installation: revision fencing, session close, TTL.
- [~] Middleware/interceptor: ordered chain с authentication, timeout и bounded failures готов; gateway interceptor wiring pending.
- [x] R04: refresh cancellation/timeout, bounded material, no store update on cancel.

**Выход M4:** `[~]` create→attach→request→rotate→detach→deny integration test проходит на fakes; real proxy/external stores blocked.

## 8. M5 — Gateway application и durable lifecycle

- [x] Новые use cases вместо бизнес-логики в `httpapi`: новый `cmd/quarshell-gateway` работает на lifecycle store и application HTTP; старый `httpapi`, legacy command и их integration/e2e tests удалены.
- [x] Entity repositories/transactions, idempotency/outbox/reconciler, recovery: memory, PostgreSQL и SQLite stores, leases/retries/DLQ и recovery готовы.
- [x] Workspaces/RBAC/templates/quotas/services/forwards/settings/provider CRUD: все перечисленные use cases готовы.
- [~] Все 83 public RPC поверх use cases: workspace/sandbox, policy lifecycle/SSE, settings, templates, quotas, providers, services, forwards, secrets metadata/handles, gateway info/health, whoami и generic events готовы; Start/Stop sandbox и provider CRUD/profiles готовы; policy chunks и tokens готовы; legacy `httpapi` удалён; gRPC surface реализует Health/Info/CurrentUser, workspace, sandbox, policy chunks, templates, settings, providers и services; unary/stream interceptors проверяют статический `QUARSHELL_TOKEN`, но per-user/sandbox claims и scope authorization pending; tokens/exec/SSH/streams/peer остаются Unimplemented. `TestEveryPublicOpenShellRPCHasFixture` сверяет все 83 generated handlers со status fixtures; functional fixtures per RPC pending.
- [~] Policy drafts/history/analysis, hot updates, observed activation: drafts/history/active/analysis/observed и SSE hot-update stream готовы; runtime activation wiring pending.
- [x] Config admission deadline; stale instance/generation fence: `admission.Gate`, sandbox/policy observe fencing.
- [x] Relay sessions, durable logs/events/cursor; R03 combined watch: combined watch, event logs, session registry и network relay/reconnect готовы.
- [~] Peer RPC, ownership leases, split-brain/reconnect, multi-gateway: registry, fencing, owner/validate/join/claim/heartbeat/release HTTP и manager с takeover готовы; relay/report RPC отдают fail-closed 501; shared lease store pending.

**Выход M5:** mutation → crash → restart не теряет намерение и не создаёт дубль; каждый public RPC имеет fixture.

## 9. M6 — Compute backend completion

- [~] **M6a Docker/Podman:** Docker lifecycle, resources, rootless config, PTY, logs и daemon reconcile готовы; hosted Linux Docker и rootful/rootless Podman E2E PASS, включая userns/binds и egress probes (allowed CONNECT, policy TCP/UDP deny, host-gateway bypass deny; см. hosted evidence). Общий Docker/Podman Engine driver реализует filtered `PlatformEventStream`; GitHub run `39684405826` подтвердил Docker `exec_create` и Podman `exec`. Полный backend parity и host-level socket/DNS/UDS bypass pending.
- [~] **M6b Kubernetes:** adapter и `--runtime kubernetes` config plumbing готовы; RBAC, namespace, NetworkPolicy, GPU/CDI, logs/exec/watch, cleanup и live cluster lane pending.
- [~] **M6c VM (6–10 недель):** local Apple Silicon `task vm:live-darwin` passed `TestLiveMicroVMLifecycle`; it verifies private isolation descriptor publication, generation preservation on worker adoption and rotation on explicit Stop/Start. Evidence: `plans/evidence/runs/vm-hvf-local-2026-10-22.md`. The scope is all 29 OpenShell v0.1.4 VM parity rows plus one supplemental host-supervisor bootstrap gap (30 inventory entries: 2 implemented, 22 partial, 2 blocked, 2 unsupported, 2 open), not module extraction alone. Host proxy now reads a mode-0600 auth bundle bound to the descriptor; initial session rotation 1 matches upstream, while subsequent rotation and backend attach/confirm/running remain unimplemented. Runtime validation remains fail-closed pending live Gateway→VM isolation acceptance. Hosted VM package tests and Windows-compatible build/tests passed in [platform run](https://github.com/KLORQ/quarshell/actions/runs/39756514346); live KVM/HVF were skipped because standard hosted runners lack hypervisor access. Signing, GPU/VFIO, KVM and G04 remain open.
- [~] **M6d Windows MXC (6–10 недель):** GitHub-hosted Windows build/test and runtime capability probe added. `quarshell-driver-mxc` is still a skeleton; MXC behavior requires a supported Windows Insider host and `wxc-exec.exe`/`IsoSessionApp.dll`.
- [ ] Закрыть G04 полным VM lifecycle, а не удалением проверки.

**Выход M6:** один backend contract suite + platform conformance для каждого пакета.

## 10. M7 — SDK, CLI, TUI, prover, agent tooling

- [~] Go SDK: пакет и integration tests зелёные; нужна проверка полного parity по typed errors/cancel/auth refresh/pagination/streaming.
- [~] CLI: команды и storage/TUI tests зелёные; нужна сверка полного дерева с upstream inventory.
- [~] TUI: базовые workflows и tests есть; нужна сверка с upstream inventory.
- [~] Python/TS bindings: Python 23/23 tests + wheel build PASS (`grpcio 1.84.0`); TS build PASS; real gateway curated scenarios pending.
- [~] Go prover: bounded containment, risk analysis, counterexamples, Z3 SMT-LIB plumbing с fallback; full symbolic model pending.
- [~] EN/RU Gateway quickstarts приведены к проверенному Go/Compose запуску; описаны общий статический token для HTTP/gRPC, public Health и отсутствие TLS/per-user identity. Hosted recheck pending. Остальные upstream примеры и public agent skills (сейчас только policy-advisor) требуют parity review.

**Выход M7:** все upstream сценарии выполнимы через новые клиенты; `Unsupported`/`Inconclusive` не разрешают policy.

## 11. M8 — Чистая сборка и удаление старой реализации

- [~] Старый `httpapi`, legacy command и stale duplicate worktrees удалены; оставшиеся старые runtime/driver packages ещё используются или не разобраны.
- [x] `.gitignore` закрывает локальные credentials и private-key файлы; Gitleaks CLI v8.30.1 проверил всю сведённую историю и рабочее дерево без совпадений; автоматический full-history scan запускается на push/PR ([GitHub run](https://github.com/KLORQ/quarshell/actions/runs/39694038181)).
- [!] GitHub Secret Scanning/push protection не включены для private `KLORQ/quarshell`; проверить organization plan/Secret Protection перед активацией.
- [x] Удалена непереносимая локальная `opencode.jsonc`; absolute developer paths из migration review artifacts заменены repo-relative references.
- [x] `task clean:generated` удалил около 3.0 GiB ignored build/cache output, включая Rust `target`, сохранив pinned upstream source.
- [x] История сведена к одному root snapshot commit на remote и локально; backup refs и reflogs удалены, `git gc --prune=now` выполнен.
- [x] Security and acceptance recheck на snapshot `2d0cd825861a48d8f6ee9ff42751bbaa1d86aced`: Gitleaks full-history PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39700612730)); Linux build/test, Docker/Podman E2E, Gateway image + Compose clean install and strict docs PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39700634881)); macOS build/test PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39701243308)); signing skipped because secrets are not configured.
- [x] GitHub Actions обновлены на Node 24-compatible releases с immutable full-SHA pins; checkout credentials не сохраняются; release permissions ограничены до `contents: read` для build и `contents: write` только для tag-only publish. Linux build/test + Docker/Podman E2E и macOS build/test прошли ([Linux](https://github.com/KLORQ/quarshell/actions/runs/39692425422), [macOS](https://github.com/KLORQ/quarshell/actions/runs/39692425519)).
- [~] Часть legacy commands/state imports удалена; env aliases/dual schemas и функции вне baseline ещё не вычищены.
- [~] Gateway image и Compose clean install проходят локальную и GitHub-hosted проверку; default HTTP/gRPC command, persistent SQLite, auth-free liveness и Docker healthcheck работают ([latest acceptance run](https://github.com/KLORQ/quarshell/actions/runs/39700634881)); Helm chart, 10-way cross-build, in-workflow manifest/SHA256/SBOM verification and artifact upload PASS ([release run](https://github.com/KLORQ/quarshell/actions/runs/39692425723)). Registry image publish, generated clients and signed releases pending.
- [ ] Закрыть все functional и platform acceptance rows, включая peer/VM/MXC/prover/GPU (блокировано live lanes).
- [x] Каждый module собирается с `GOWORK=off` через local file module proxy (20/20).
- [~] Published dependency gate (`bash scripts/check-published-deps.sh`) не проходит: зависимостям нужны недостающие checksums/обновление go.mod, а `quarshell-core@v0.3.0` не совпал с публичным checksum database; release tags/зависимости надо сверить и выпустить согласованный набор до consumer acceptance.
- [~] Binaries/SBOM/checksums artifact with in-workflow verification PASS, signatures, images, upgrade и clean install pending.
- [~] Final coverage report: 0 open/partial/blocked rows недостижим без live lanes; standalone 20/20.

**Выход M8:** релиз новой реализации, а не только зелёный `go test ./...` одного workspace.

## 12. Backlog исправлений из ревью

| ID | Проблема | Первый этап | Статус |
| --- | --- | --- | --- |
| R01 | Нет upstream-бюджета размера policy | M2 | `[x]` |
| R02 | `null` стирает absent/invalid | M2 | `[x]` |
| R03 | Закрытие platform-events завершает combined watch | M5 | `[~]` |
| R04 | Credential refresh игнорирует context | M4 | `[x]` |
| R05 | Evidence registry противоречит рабочей копии | M0 | `[~]` |
| G01 | Runtime isolation уже upstream | M3 | `[ ]` |
| G02 | Stable placeholders/token grants не end-to-end | M4 | `[ ]` |
| G03 | Peer RPC — отказы | M5 | `[ ]` |
| G04 | Gateway→VM всегда ошибка | M6c | `[ ]` |
| G05 | Process monitor не видит всех потомков | M3 | `[~]` |
| G06 | Неполная configuration admission | M5 | `[ ]` |
| G07 | Scope шире текущих модулей | M6–M7 | `[ ]` |

## 13. Ближайшие конкретные задачи

1. [x] Создать baseline feature matrix и включить отсутствующие IsolationBoundary RPC и prover/MXC/VFIO inventory.
2. [~] Исправить evidence R05: registry/gate/generated `STATUS.md` есть; OS-POLICY-SCHEMA получил воспроизводимую ссылку на тест и digest; остальные 28 строк остаются `unverified` до получения acceptance evidence.
3. [x] Утвердить ADR: compute/isolation separation; module DAG; state/outbox; runtime security; native dependencies; no legacy compatibility.
4. [x] Создать новые `core/policy/spi` contracts и compiled examples, затем bounded policy corpus R01/R02: R01/R02 проходят локально и против pinned upstream oracle.
5. [~] Выполнить Linux launcher/broker spike до обещаний полного Go runtime: launcher/child, Landlock и admission есть; live kernel lane blocked.
6. [ ] Построить первый вертикальный сценарий: CLI create → gateway transaction → Docker → Go supervisor/isolation → enforced policy → credential request → stop/delete/recovery.
7. [ ] Расширять путь по matrix, каждый раз удаляя заменённый old path.

## 14. Открытые внешние gates

- [~] GitHub-hosted Linux build/test и Docker/Podman egress probes PASS; raw-socket/seccomp и DNS/UDS host-level bypass probes pending.
- [x] Docker/Podman native Linux lane на GitHub-hosted `ubuntu-24.04`: rootful/rootless E2E PASS, включая userns/binds и backend egress probes ([run](https://github.com/KLORQ/quarshell/actions/runs/39679621940), [evidence](evidence/runs/hosted-linux-acceptance-2026-10-22.md)).
- [x] Последний security-hardened GitHub-hosted recheck на `master`: Linux полный workspace/test gate и Docker/Podman lifecycle+event E2E PASS; macOS `build-test` PASS ([Linux](https://github.com/KLORQ/quarshell/actions/runs/39692425422), [macOS](https://github.com/KLORQ/quarshell/actions/runs/39692425519)). Checkout credentials не сохраняются; write permission есть только у tag-only publish.
- [ ] Kubernetes real-cluster lane (blocked: cluster + runtime image + TLS).
- [~] GitHub-hosted KVM/HVF and Windows capability probes (`.github/workflows/platform-backends.yml`); live acceptance still requires KVM/HVF access or MXC runtime.
- [ ] Signed macOS components (blocked: Apple certificate/notarization secrets).
- [ ] Windows MXC driver and real runtime acceptance (blocked: implementation + supported Windows Insider host + MXC runtime).
- [ ] GPU/VFIO hardware (blocked: physical hardware).
- [ ] Published consumer без local replace и чистый install (blocked: registry/images).
