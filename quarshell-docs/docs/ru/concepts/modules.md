# Модули проекта

Quarshell состоит из 10 независимых Go-модулей, а также TypeScript UI, Python SDK, документации и репозитория workspace. API остаётся нестабильным: используем возрастающие версии `v0.x.y`. Список модулей описывает границы кода, а пробелы OpenShell parity перечислены в [реестре совместимости](../reference/openshell-compatibility.md).

| Модуль | Назначение |
| --- | --- |
| [`quarshell-cli`](https://github.com/KLORQ/quarshell-cli) | CLI `quarshell`, пользовательские команды, локальный UX и GUI helpers. |
| [`quarshell-core`](https://github.com/KLORQ/quarshell-core) | Доменные типы, credential primitives, общие порты, изоляционные контракты и logging helpers. |
| [`quarshell-policy`](https://github.com/KLORQ/quarshell-policy) | Policy parser/evaluator, provider profiles, prover и conformance logic. |
| [`quarshell-driver`](https://github.com/KLORQ/quarshell-driver) | Compute SPI, Docker/Podman/Kubernetes drivers, host isolation и process supervisor. |
| [`quarshell-driver-vm`](https://github.com/KLORQ/quarshell-driver-vm) | MicroVM, worker, guest transport, KVM/HVF и VM runtime orchestration. |
| [`quarshell-gateway`](https://github.com/KLORQ/quarshell-gateway) | Go backend: API/RPC, OIDC, orchestration, state, credential storage и UI read models. |
| [`quarshell-runtime`](https://github.com/KLORQ/quarshell-runtime) | Guest init, supervisor, hardening и sandbox image helpers. |
| [`quarshell-proxy`](https://github.com/KLORQ/quarshell-proxy) | Egress enforcement, credential mediation и аудит. |
| [`quarshell-sdk`](https://github.com/KLORQ/quarshell-sdk) | Публичный Go-клиент Gateway RPC. |
| [`quarshell-proto`](https://github.com/KLORQ/quarshell-proto) | Protobuf контракты и generated Go/Connect/gRPC/TypeScript bindings. |

## Другие репозитории

| Репозиторий | Назначение |
| --- | --- |
| [`quarshell-ui`](https://github.com/KLORQ/quarshell-ui) | TypeScript/React browser console; Gateway остаётся единственным Go backend. |
| [`quarshell-python`](https://github.com/KLORQ/quarshell-python) | Python SDK. |
| [`quarshell-deploy`](https://github.com/KLORQ/quarshell-deploy) | Helm и deployment assets. |
| [`sandbox.dev`](https://github.com/KLORQ/sandbox.dev) | MkDocs пользовательская и техническая документация. |
| [`quarshell-workspace`](https://github.com/KLORQ/quarshell-workspace) | Pins, workspace tooling, планы и integration orchestration. |

Путь запроса: CLI/UI/SDK → Gateway → compute driver → Docker/Podman или VM. Proxy проверяет egress, policy задаёт правила, runtime управляет процессами внутри sandbox. UI обращается только к авторизованному Gateway API.
