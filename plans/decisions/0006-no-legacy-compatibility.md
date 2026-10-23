# ADR-0006: Нет совместимости со старым Quarshell/Quarshell

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

Старые API, конфиги, env aliases, state layout, images и дополнительные функции
Quarshell/Quarshell не являются требованием. Сохранение их параллельно создаёт
dual readers и скрытые fallback-пути.

## Решение

- Функциональный эталон — только pinned OpenShell v0.1.4.
- Внешний OpenShell wire contract сохраняется как публичный протокол.
- Старые Quarshell API/state/domain не адаптируются.
- Нет `legacy`, `compat-engine`, временных fallback на старые binaries или
  readers старого state.
- Имена `openshell` в generated wire/schema и attribution разрешены: это
  исходный контракт, а не legacy execution.
- Пользовательские данные на хосте не удаляются новым installer автоматически.
- Старый код удаляется по функциональным владельцам после замены target scope.

## Последствия

- Чистая установка использует новый state layout.
- Dual API, env fallbacks, imports aliases и adoption старых resources
  исключены.
- Документация, CLI и packaging используют одно имя Quarshell.
- Удаление исходников не означает удаление пользовательских sandbox volumes.
