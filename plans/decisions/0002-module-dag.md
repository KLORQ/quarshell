# ADR-0002: DAG модулей и допустимые зависимости

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

Монолитный gateway и общий `core` приводят к циклическим и transitively
протянутым зависимостям: transport, runtime, драйверы и proto смешиваются.

## Решение

Целевые модули и направление зависимостей:

```text
quarshell-core
  ├── quarshell-policy
  ├── quarshell-spi
  ├── quarshell-telemetry
  ├── quarshell-policy
  ├── quarshell-credentials
  ├── quarshell-proxy
  ├── quarshell-isolation
  ├── quarshell-supervisor
  ├── quarshell-driver
  ├── quarshell-driver-vm
  ├── quarshell-driver-mxc
  ├── quarshell-gateway
  ├── quarshell-sdk
  ├── quarshell-cli
  └── quarshell-policy/prover
quarshell-proto — без доменных зависимостей
```

- `core` не содержит protobuf, SQL, HTTP, глобального logging и registries.
- `sdk` не импортирует server/backend.
- `gateway` library не импортирует VM FFI или Windows APIs.
- Builtin backends собираются в явных composition roots.
- Новый Go module создаётся только для независимого consumer, release,
  платформы или тяжёлой зависимости.

## Последствия

- Один feature contract может жить в нескольких модулях без общего «свалочного»
  пакета.
- `GOWORK=off` сборка каждого модуля — обязательный release gate.
- Import cycle или SDK→server dependency блокирует merge.
