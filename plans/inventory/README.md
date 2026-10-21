# Inventory

Машиночитаемый inventory строится из pinned checkout
`tools/upstream/openshell-v0.1.4` и таблицы функционального контракта нового
плана (`plans/openshell-go-2026-10-21/PLAN.md`, раздел 2).

```bash
python3 tools/generate-openshell-inventory.py
```

Выходные файлы:

- `openshell-v0.1.4-inventory.json` — crates, RPC, CLI, config, TUI, tests,
  docs, installers, текущий module DAG;
- `features.yaml` — feature matrix и coverage records.

Сгенерированные `*.json` и `*.yaml` не коммитятся: source of truth — upstream
checkout и план. Перегенерация обязательна после изменения плана или baseline.

Статус: `unverified`. Это inventory, а не доказательство поведения. Для каждой
строки нужен отдельный acceptance fixture.
