# Карта всех исходных компонентов OpenShell

Baseline: локальный `tools/upstream/openshell-v0.1.4`, 37 crates. Все строки ниже обязательны к функциональной классификации; это назначения нового кода, не предложение оборачивать старый Rust.

| Upstream crate | Новый владелец (`quarshell-*`) | Что перенести / проверить |
|---|---|---|
| `openshell-binary-identity` | isolation | Trusted executable identity, procfs, PID reuse, interpreter semantics |
| `openshell-bootstrap` | CLI / SDK | Gateway metadata, local auth state, mTLS bundles, OIDC lifecycle |
| `openshell-cli` | CLI | Полное command/flags/output/exit-code дерево |
| `openshell-conformance` | conformance tooling | Backend-independent scenarios и differential runner |
| `openshell-core` | core / policy | Domain/config/errors; разделить чистую семантику и transport |
| `openshell-driver-db-credstore` | credentials/db | Encrypted credential persistence, rotation, CRUD |
| `openshell-driver-docker` | driver/docker | Containers, networks, resources, workspaces, lifecycle |
| `openshell-driver-kubernetes` | driver/kubernetes | Pods, RBAC, service accounts, volumes, events, resources |
| `openshell-driver-kubernetes-secrets` | credentials/kubernetes | Secret storage, identity/namespace isolation |
| `openshell-driver-mxc` | driver-mxc | Windows isolation sessions, filesystem/network/process mapping |
| `openshell-driver-podman` | driver/podman | Rootful/rootless lifecycle, userns, host routes, resources |
| `openshell-driver-vault` | credentials/vault | Vault-compatible storage/auth/refresh/outage |
| `openshell-driver-vm` | driver-vm | KVM/HVF, OCI/rootfs, host/guest boundary, native dependency |
| `openshell-extension-core` | spi + proto transport | Extension identity, version negotiation, JWT rotation, TLS |
| `openshell-gateway` | gateway/cmd | Explicit composition of server and selected compute drivers |
| `openshell-gateway-interceptors` | gateway/extensions | Request interception, ordered transformation and authorization |
| `openshell-isolation-interface` | spi/isolation | Isolation contract independent of compute allocation |
| `openshell-ocsf` | telemetry/ocsf | Typed audit builders/formatters; loss and redaction handling |
| `openshell-otel` | telemetry/otel | Resource/traces/OTLP lifecycle and bounded shutdown |
| `openshell-otel-test-support` | telemetry tests | Local collector fixtures; не production dependency |
| `openshell-policy` | policy/evaluate + isolation | Runtime decision semantics and enforcement mapping |
| `openshell-policy-schema` | policy/schema | Bounded YAML/JSON, authored types, validation/presence/defaults |
| `openshell-prover` | prover | Symbolic model, reachability, containment, validated witnesses |
| `openshell-prover-cli` | prover/cmd | Local verification, stable JSON reports and exit codes |
| `openshell-providers` | providers + credentials | Profiles/catalog/bindings; получение секретов отдельно |
| `openshell-sandbox` | isolation | Capability-restricted launcher, syscall mediation, process/FD lifecycle |
| `openshell-sandbox-backend` | isolation/transport + proto | IsolationBoundary protocol, authenticated exchange/mediation |
| `openshell-sdk` | SDK | Go client replaces Rust implementation role; equivalent workflows |
| `openshell-server` | gateway/application + storage + transport | All API use cases, durable lifecycle, auth, registry |
| `openshell-server-macros` | gateway/transport authorization metadata | Declarative auth rules/code generation; Rust macro как механизм не переносится |
| `openshell-supervisor` | supervisor | Gateway sessions, config admission, runtime control composition |
| `openshell-supervisor-middleware` | proxy/extensions | Remote middleware registry/chain execution |
| `openshell-supervisor-middleware-builtins` | proxy/middleware | Builtin middleware semantics and failure policies |
| `openshell-supervisor-network` | proxy | TCP/HTTP/TLS/L7/provider credential injection |
| `openshell-supervisor-process` | supervisor/process + isolation | Managed process lifecycle, namespace and bypass observation |
| `openshell-tui` | CLI/tui | Monitoring and interactive management workflows |
| `openshell-vfio` | driver-vm/vfio | PCI/GPU device preparation, ownership, cleanup/recovery |

## Поверхности вне crates

| Каталог upstream | Работы |
|---|---|
| `proto/` + sandbox backend proto | 110 RPC известных runtime сервисов, message variants, errors, options/auth |
| `sdk/go/` | Сопоставить public Go SDK operations и semantic types, packaging и examples |
| `python/openshell/` | Thin Python SDK/CLI packaging к новой Go реализации |
| `sdk/typescript/` | Native Connect API и curated client, generated types |
| `providers/` | Каждый provider profile и credential strategy в fixture corpus |
| `deploy/`, packaging manifests | Images, Helm, system/package install, platform matrix |
| `examples/`, `skills/`, docs | Пользовательские сценарии и installable agent workflows |
| `tests/`, `e2e/` | Oracle scenarios; не считать upstream tests тестами новой реализации |

Перечень crates не заменяет inventory функций внутри каждого компонента. M0 раскладывает каждую строку на сценарии, методы, конфиги и host profiles.
