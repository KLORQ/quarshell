# Architecture decisions

| ADR | Решение | Статус |
| --- | --- | --- |
| [0001](0001-compute-isolation-separation.md) | Compute и Isolation — разные контракты | Accepted |
| [0002](0002-module-dag.md) | DAG модулей и допустимые зависимости | Accepted |
| [0003](0003-state-outbox.md) | Transactional state, idempotency и outbox | Accepted |
| [0004](0004-runtime-security.md) | Runtime security без Rust fallback | Accepted |
| [0005](0005-native-dependencies.md) | Native dependencies — отдельные gates | Accepted |
| [0006](0006-no-legacy-compatibility.md) | Нет совместимости со старым Quarshell/Quarshell | Accepted |
| [0007](0007-backend-config-scope.md) | Typed config: implemented или fail-closed reject; behavior в M6 | Accepted |

ADR фиксируют решения плана. Изменение решения оформляется новым ADR, а не
правкой принятого.
