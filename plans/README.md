# Рабочие планы Quarshell

Принят план полного переноса функциональности OpenShell на модульную Go-реализацию.
Baseline: OpenShell `v0.1.4`, commit `5a7728c304b5528e954e9ae900cf21f84e1a12b1`.
Namespace: `github.com/KLORQ`.

## Что здесь лежит

| Документ | Назначение |
| --- | --- |
| [REMAINING.md](REMAINING.md) | Актуальный review и приоритетный остаток OpenShell parity |
| [baseline/BASELINE.md](baseline/BASELINE.md) | Замороженный baseline и состояние рабочих деревьев |
| [inventory/README.md](inventory/README.md) | Генерация inventory из pinned checkout и плана |
| [evidence/README.md](evidence/README.md) | Машиночитаемая evidence matrix и правила pass |
| [ci-matrix.md](ci-matrix.md) | CI lanes, runners и локальные macOS-замены |
| [backlog/RG-ACCEPTANCE.md](backlog/RG-ACCEPTANCE.md) | R/G/A backlog с acceptance |
| [decisions/README.md](decisions/README.md) | Принятые архитектурные решения |
| [conformance/corpus/manifest.yaml](conformance/corpus/manifest.yaml) | Differential/conformance corpus; upstream oracle: `task conformance:upstream` |
| [openshell-go-2026-10-21/PLAN.md](openshell-go-2026-10-21/PLAN.md) | Полный план: архитектура, этапы M0–M8, приёмка |
| [openshell-go-2026-10-21/REVIEW.md](openshell-go-2026-10-21/REVIEW.md) | Ревью: R01–R05, G01–G07, A01–A04 |
| [openshell-go-2026-10-21/upstream-components.md](openshell-go-2026-10-21/upstream-components.md) | Карта 37 upstream crates |
| [openshell-go-2026-10-21/rpc-inventory.csv](openshell-go-2026-10-21/rpc-inventory.csv) | Инвентарь RPC |
| [openshell-go-2026-10-21/review-manifest.json](openshell-go-2026-10-21/review-manifest.json) | Состояние checkout на дату ревью |
| [openshell-go-2026-10-21/source-sha256.txt](openshell-go-2026-10-21/source-sha256.txt) | Хеши исходников рабочей копии |
| [openshell-go-2026-10-21/verification.txt](openshell-go-2026-10-21/verification.txt) | Выполненные проверки и ограничения |
| [openshell-go-2026-10-21/examples](openshell-go-2026-10-21/examples) | Компилируемые design patterns и policy probe |

## Как ведётся работа

1. Актуальный engineering backlog и приёмка ведутся в `REMAINING.md`.
   Исторические изменения и первоначальный план сохраняются в
   `openshell-go-2026-10-21/` и `evidence/`.
2. `pass` ставится только при наличии воспроизводимой команды и артефакта.
   Всё остальное — `unverified`, `partial` или `blocked`.
3. Обратная совместимость с предыдущими pre-stable API не поддерживается;
   новый namespace — `github.com/KLORQ`. Внешний контракт —
   только pinned OpenShell baseline.
4. Источники OpenShell, evidence reports, ADR и conformance fixtures сохраняются
   как provenance; не удалять их при очистке локальных build outputs.
