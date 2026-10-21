# ADR-0003: Transactional state, idempotency и outbox

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

Whole-state JSON/JSONB store с одним writer ограничивает concurrency, размер
состояния и active-active gateway. Сетевой side effect внутри DB transaction
нельзя сделать атомарным.

## Решение

- Entity repositories: sandbox, operation, workspace, membership, policy
  revision, provider binding, credential metadata, session, event/outbox.
- SQLite для single-node, PostgreSQL для distributed профиля.
- Create/delete/provider mutations используют idempotency key + request hash.
- Desired state и outbox пишутся одной транзакцией.
- Worker после commit захватывает lease с fencing token, наблюдает backend,
  выполняет ensure и CAS observed state.
- Ambiguous timeout разрешается поиском ресурса по operation/sandbox ID.
- Reconciler доводит desired state после crash.

## Последствия

- DB transaction не включает сетевой вызов engine.
- Exactly-once удалённый side effect не обещается; обещается идемпотентность и
  явная reconciliation.
- HA требует ownership lease с fencing; replica без fencing не считается HA.
- Secrets не попадают в общий JSON snapshot.
