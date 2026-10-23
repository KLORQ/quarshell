# Evidence matrix

Единственный машиночитаемый источник статуса готовности:
[`matrix.yaml`](matrix.yaml).

Правила:

- `result` — одно из `pass`, `fail`, `blocked`, `unverified`.
- `partial` запрещён как финальный статус: строка остаётся `unverified` или
  `blocked`, пока нет полного acceptance.
- `pass` требует `backend`, `host_profile`, `command`, `artifact`,
  `evidence_level` и 64-символьный `source_digest`.
- `artifact` — путь в репозитории или URL; локальный путь должен существовать.
- `evidence_level` — `static`, `unit`, `integration`, `target-host`, `release`.
- Новая реализация начинает каждую строку с `unverified`. Старый зелёный fixture
  не переносится автоматически.
- Документация и release summary должны генерироваться из этой матрицы, а не
  поддерживаться отдельно.

Сгенерированный статус: [`STATUS.md`](STATUS.md). Его нельзя править вручную.

Исторический Go source qualification snapshot сохранён в
[`runs/go-source-qualification-2026-10-19.json`](runs/go-source-qualification-2026-10-19.json).
Он отражает отдельную проверку до модульного переноса и не является текущим
acceptance result.

The latest standalone dependency check and its checksum mismatch are recorded
in [`runs/published-dependency-gate-2026-10-22.md`](runs/published-dependency-gate-2026-10-22.md).

```bash
task evidence:check
task evidence:report-check
```

Валидатор блокирует дубли, неизвестные feature IDs, неполный `pass`, битые
artifact-ссылки и несовпадение baseline SHA.
