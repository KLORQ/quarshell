# ADR-0005: Native dependencies — отдельные gates

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

Перенос логики на Go не устраняет ОС, container engines, hypervisors, Z3, GPU
drivers и системные API. libkrun остаётся native dependency даже при вызове через
FFI без cgo.

## Решение

- Каждый native компонент получает отдельное архитектурное решение, pin и
  packaging gate.
- Native dependency не скрывается за «чистым Go».
- Поддержанная платформа и версия фиксируются в platform matrix.
- Неподдержанная комбинация возвращает явный отказ до создания ресурсов.
- Upstream Rust crates не используются как production dependency.

## Последствия

- Release artifacts включают SBOM, licenses и provenance для native частей.
- VM/MXC/GPU/prover lanes требуют выделенного оборудования и CI.
- Полный отказ от native dependencies — отдельный объём работ, не следствие
  переноса.
