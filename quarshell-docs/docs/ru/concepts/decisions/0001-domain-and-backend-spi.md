<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# ADR 0001: доменная модель и compute backend SPI

- **Статус:** принято и реализовано; оставшиеся OpenShell gaps отслеживаются в compatibility matrix
- **Дата:** 2026-10-16
- **Ответственные за решение:** сопровождающие Quarshell
- **Область:** compute backends и оркестрация Gateway

## Контекст

В driver package определены собственный request и контракт `Backend`, типизированные
backend identity и capabilities, а также optional extension interfaces. Все
встроенные реализации напрямую регистрируют этот контракт; Gateway выбирает
только native Backends. Remote ComputeDriver plugins реализуют тот же Backend
contract через pinned OpenShell protocol. VM backend пока fail-closed при
Gateway runtime validation: его host-supervisor и isolation lifecycle ещё не
завершены. Бывший flat `Spec`/`ComputeDriver` compatibility API удалён.

## Решение

1. **Продукт владеет доменным контрактом.** Оркестрация использует
   `BackendCreateRequest`, `Backend`, `BackendInfo` и типизированные доменные
   значения. Transport messages, внешние protobuf-типы и запросы container
   engine не входят в этот контракт.
2. **Обязательный lifecycle остаётся небольшим.** `Backend` отвечает за
   identity, объявление capabilities, валидацию и выполнение create, start,
   stop, delete, list и inspect. Необязательное поведение описывают узкие
   extension interfaces, например `InteractiveExec`, `FileTransfer`,
   `ServiceForward`, `LogAccess` и runtime extensions. Backend реализует только
   реально поддерживаемые функции.
3. **Capability должна совпадать с реализацией.** Вызывающая сторона получает
   extension через `driver.RequireExtension[T]`. Helper проверяет и
   объявленную capability, и наличие соответствующего метода; при отсутствии
   любого из них возвращается типизированный `UnsupportedError`. Нельзя молча
   отключать запрошенное поведение безопасности или lifecycle.
4. **Сначала валидация, затем фиксация состояния.** Gateway разрешает и
   валидирует create request до записи durable lifecycle state и расходования
   одноразового upload token. Secrets передаются только на короткую операцию и
   не копируются в durable create records.
5. **Использовать прямой модульный backend-контракт.** Все встроенные реализации
   напрямую регистрируют `Backend` и владеют типизированной create-моделью.
   Gateway разрешает только native factory и возвращает её ошибку без fallback.
   Предыдущее предложение о migration adapter заменено решением о прямом
   переносе от 2026-10-21; очистка неиспользуемого flat API отслеживается в B0.6.
6. **Сохранять закреплённый wire contract.** Сгенерированные OpenShell schema и
   Gateway implementation сохраняют точные upstream service, field и package
   identifiers. Gateway methods напрямую сопоставляют эти requests с доменными
   операциями. Собственные имена domain и API не наследуют upstream wire
   identifiers. Неподдерживаемые операции возвращают типизированный
   unsupported-результат на границе протокола.
7. **Клиенты независимы.** CLI и browser UI — отдельные API clients. CLI может
   включать terminal presentation code и вызывать SDK; browser UI использует
   typed browser client и Gateway API. Gateway не импортирует CLI или UI, а
   browser UI не запускает CLI.

## Пример контракта

Ниже — фрагмент реализованного Go SPI. Тип настроек принадлежит конкретному
backend package; общему request не нужен сериализованный `driver config`:

```go
type Backend interface {
    ID() BackendID
    Capabilities(context.Context) (BackendInfo, error)
    ValidateCreate(context.Context, BackendCreateRequest) error
    Create(context.Context, BackendCreateRequest) (Handle, error)
    Start(context.Context, core.ID) error
    Stop(context.Context, core.ID) error
    Delete(context.Context, core.ID) error
    List(context.Context) ([]Info, error)
    Inspect(context.Context, string) (Info, error)
}

type BackendCreateRequest struct {
    SandboxID core.ID
    Name      string
    Workspace WorkspaceTarget
    Workload  WorkloadSpec
    Resources ResourceLimits
    Mounts    []Mount
    Network   NetworkSpec
    Runtime   RuntimeAssets
    Config    any // типизирован настройками backend, это не wire-format map
}

transfer, err := driver.RequireExtension[driver.FileTransfer](
    ctx, backend, driver.CapabilityFileTransfer, "copy into sandbox",
)
if err != nil {
    return err // сохранить Unsupported; не утверждать, что операция выполнена
}
return transfer.CopyTo(ctx, sandboxID, source, destination)
```

Настоящий `BackendCreateRequest` также содержит sandbox name и флаг admission
небезопасного mount. Значения runtime и network временны. Реализация вызывает
`ValidateCreate` до создания backend resources и отклоняет входные данные,
которые не может безопасно обработать.

## Структура модулей и адаптеров

Целевые границы ответственности:

```text
core/                 доменные значения, policy, validation и типизированные ошибки
proto/                схемы product API и сгенерированные product clients
driver/               Backend SPI и Docker, Podman, MicroVM adapters
runtime/              guest init, supervisor и guest protocol
proxy/                egress enforcement и credential mediation
providers/             provider profiles и credential refresh validation
gateway/               auth, оркестрация, durable state и API adapters
sdk/                   Go и Python API clients
cli/                   команды, вызовы SDK и terminal presentation
ui/                    browser app и typed browser API client
```

Это границы владения кодом, а не требование немедленно разделить каждый
repository. Текущие repository и Go module paths пока используют имена Quarshell;
этапы переименования описаны в
[плане переименования и релиза](../../reference/renaming-and-release.md).

Gateway resolver в `quarshell-gateway/internal/httpapi/compute.go` выбирает
зарегистрированные native backends. Обработчик OpenShell protocol один раз
преобразует запрос в модульный create request. Backend владеет typed config и
реализует доменный SPI без импорта CLI packages и transport schemas.

## Последствия

- Новый backend можно добавить за SPI без backend-specific условий в API
  clients и без включения его настроек в общую wire schema.
- У optional features есть явная capability negotiation; отсутствие функции
  обрабатывается fail-closed.
- Все Docker, Podman, Kubernetes и VM backends регистрируют модульный Backend;
  прежняя flat `Spec`/`ComputeDriver` compatibility-поверхность удалена.
- Docker, Podman и MicroVM переходят независимо и проходят собственные runtime
  acceptance gates. Текущий код и локальные тесты не доказывают полную
  совместимость функций или production readiness.
- На gRPC compatibility boundary неподдерживаемое поведение преобразуется в
  предусмотренный протоколом unsupported status. Существующие wire identifiers
  сохраняются, а собственные API используют своё пространство имён.
- Переименование modules, packages и binaries ждёт настройки целевых remotes,
  публикации replacement dependencies и успешной сборки consumer без локальных
  workspace overrides. Порядок cutover и rollback задан в release plan.

## Проверка

Решение отражено в `driver/backend.go`, Gateway backend resolver,
compatibility RPC adapter, RPC inventory и contract tests. Тесты проверяют
adapter и явный unsupported path. Production acceptance backend-ов определяется
feature matrix и реестром открытых gaps; этот ADR не меняет их статусы.
