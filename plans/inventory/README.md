# Inventory

Машиночитаемый inventory строится из pinned checkout
`tools/upstream/openshell-v0.1.4` и таблицы функционального контракта нового
плана (`plans/openshell-go-2026-10-21/PLAN.md`, раздел 2).

```bash
go run tools/cmd/quarshell-tools/*.go inventory --write
go run tools/cmd/quarshell-tools/*.go inventory --check
```

Выходные файлы:

- `openshell-v0.1.4-inventory.json` — crates, RPC, CLI, config, TUI, tests,
  docs, installers, текущий module DAG;
- `features.yaml` — feature matrix и coverage records.

Сгенерированные `*.json` и `*.yaml` коммитятся как reviewable snapshots;
source of truth — pinned upstream checkout и feature register. Перегенерация и
`--check` обязательны после изменения baseline или соответствующей матрицы.

Inventory выводится из закреплённого upstream checkout и feature register.
Source mapping подтверждает только наличие кандидата на реализацию, не parity.
Для статуса `implemented` нужны привязанные к исходнику тесты/evidence; для
`released` нужен consumer acceptance без workspace replacements. Текущий gap
plan и ограничения: [`../REMAINING.md`](../REMAINING.md).
