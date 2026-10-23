# Backlog R/G/A: acceptance

Источник: [REVIEW.md](../openshell-go-2026-10-21/REVIEW.md).
Статус всех строк — `unverified`; закрытие требует evidence в
[`../evidence/matrix.yaml`](../evidence/matrix.yaml).

## P1 исправления

### R01 — бюджет размера policy

- **Работа:** bounded reader до выделения буфера; parser budget во время
  построения AST; запрет повторных полных разборов.
- **Приёмка:**
  - `limit-1`, `limit`, `limit+1` для 4 MiB;
  - depth 64, 300000 events, 100000 nodes, 100 aliases, 10000 keys/elements;
  - большой scalar, alias bomb, merge keys;
  - превышение даёт стабильную ошибку, не panic и не неограниченную память.
- **Этап:** M2. **Статус:** `pass` (unit boundaries + corpus; 71/71 pinned upstream differential).

### R02 — null vs absent vs invalid

- **Работа:** AST-декодирование с presence; `AuthoredField[T]`; полная
  upstream-таблица запрещённых null-объектов.
- **Приёмка:** одинаковые результаты для absent, null, empty object, false,
  zero, empty list, unknown field; golden corpus против pinned upstream.
- **Этап:** M2. **Статус:** `pass` (`task conformance:upstream`, 71/71 cases против pinned upstream oracle).

### R03 — combined watch

- **Работа:** независимый lifecycle каждого источника; `nil` при закрытии;
  reconnect с cursor либо явная `Unavailable`; stop condition только по
  контракту.
- **Приёмка:** events EOF + log; events EOF + status; dropped cursor;
  reconnect; slow consumer; все источники закрыты; отмена без утечек.
- **Этап:** M5. **Статус:** `unverified`.

### R04 — credential helper cancellation

- **Работа:** context через refresh; deadline; ограниченные stdout/stderr;
  redacted error; process group/job cleanup.
- **Приёмка:** helper зависает, игнорирует TERM, порождает потомка, пишет больше
  лимита, возвращает секрет в stderr — результат ограничен по времени, памяти и
  диагностике.
- **Этап:** M4. **Статус:** `unverified`.

### R05 — evidence registry

- **Работа:** одна машиночитаемая matrix; source SHA/digest, host/backend/kernel,
  команда, exit status, дата, артефакт; генерация docs/release из неё.
- **Приёмка:** битая ссылка, устаревший SHA, пустой report, skipped mandatory
  сценарий блокируют promotion; новый код стартует с `unverified`.
- **Этап:** M0. **Статус:** `unverified` (matrix создана, gate добавлен).

## Функциональные пробелы

### G01 — runtime isolation

- **Работа:** отдельный `quarshell-isolation`: authenticated boundary, trusted
  binary identity, syscall mediation, capabilities/no_new_privs, FD lifecycle,
  подтверждение ограничений до запуска.
- **Приёмка:** untrusted workload реально ограничен после exec; нет Rust
  fallback; evidence привязан к instance и host capability set.
- **Этап:** M3. **Статус:** `unverified`.

### G02 — placeholders и token grants

- **Работа:** policy-bound handles, attachment epoch, runtime installation ACK,
  refresh/expiry/revoke, старая generation отклоняется, managed files.
- **Приёмка:** create → attach → request → rotate → detach → denied; plaintext
  fallback отсутствует.
- **Этап:** M4. **Статус:** `unverified`.

### G03 — peer RPC

- **Работа:** peer identity, session ownership, workspace authz, bounded relay,
  fencing, restart/reconnect, retry budgets.
- **Приёмка:** identity/fencing, reconnect, gateway outage, split-brain.
- **Этап:** M5. **Статус:** `unverified`.

### G04 — VM path

- **Работа:** host supervisor, authenticated host/guest boundary, worker
  lifecycle, resources, workspace ownership, KVM/HVF, rootfs, recovery, GPU/VFIO.
- **Приёмка:** gateway → VM E2E lifecycle; G04 не закрывается удалением проверки.
- **Этап:** M6c. **Статус:** `unverified`.

### G05 — process monitor

- **Работа:** launcher-owned main/exec events; finalization каждого процесса;
  descendant coverage; identity с generation/start time; loss-aware sequence.
- **Приёмка:** процессы короче scan interval, потомки, PID reuse, restart.
- **Этап:** M3. **Статус:** `unverified`.

### G06 — configuration admission

- **Работа:** единый автомат provisioning → configuration pending → accepted →
  running/failed; durable deadline; instance fence; policy hash/provider revision.
- **Приёмка:** истечение deadline запускает cleanup, а не только меняет UI.
- **Этап:** M5. **Статус:** `unverified`.

### G07 — полный scope

- **Работа:** MXC, prover, GPU/VFIO, credential drivers, extensions,
  provider-managed files, SDK/TUI, packaging.
- **Приёмка:** каждая строка имеет executable evidence на целевой платформе.
- **Этап:** M6–M8. **Статус:** `unverified`.

## Архитектурные проблемы

### A01 — whole-state storage

- **Решение:** entity repositories, transactions, unique constraints, CAS,
  operation log, outbox.
- **Приёмка:** mutation → crash → restart не теряет намерение и не создаёт дубль;
  active-active не обещается без fencing.
- **Этап:** M1/M5. **Статус:** `unverified`.

### A02 — бизнес-логика в transport

- **Решение:** transport → application → domain/ports.
- **Приёмка:** нет orchestration в `httpapi`; domain не импортирует proto/gRPC.
- **Этап:** M1/M5. **Статус:** `unverified`.

### A03 — именование

- **Решение:** одно имя Quarshell; upstream wire identifiers и attribution —
  единственные исключения.
- **Приёмка:** в новом коде нет `quarshell-*`, `QUARSHELL_*`, старых env fallback.
- **Этап:** M1/M8. **Статус:** `unverified`.

### A04 — platform matrix

- **Решение:** feature × backend × OS/arch × privilege mode × kernel/engine.
- **Приёмка:** неподдержанная комбинация отказывает до создания ресурсов;
  обязательная комбинация остаётся blocker до host acceptance.
- **Этап:** M0–M8. **Статус:** `unverified`.
