# Остаток переноса OpenShell

Целевой baseline Quarshell — закреплённый OpenShell `v0.1.4`. Совместимость
остаётся частичной, пока поведенческая приёмка не покрывает обязательные RPC,
providers, runtime, clients и backends. Совпадение schema и регистрация handler
не доказывают parity.

Текущие hosted root CI, Linux KVM lifecycle, nested Docker и rootful/rootless
Podman acceptance проходят. Локальные macOS/HVF lifecycle и Gateway composition
также проходят. Это закрывает старый KVM environment gate, но не все VM parity
rows.

Оставшиеся направления:

- Gateway authentication, token/OAuth lifecycle, незавершённые peer RPC и
  функциональная приёмка каждого public RPC.
- Multi-Gateway ownership/reconnect и at-least-once граница stream cursor.
- Linux seccomp notification, lifecycle дочерних процессов и проверки обхода
  через raw sockets/DNS/Unix sockets.
- Kubernetes acceptance на реальном cluster, полное соответствие Docker/Podman
  config и network behavior.
- Оставшиеся VM feature rows, Windows MXC runtime и hardware для GPU/VFIO.
- Поведение CLI/TUI, SDK, Python/TypeScript clients и symbolic prover.
- Установка опубликованного consumer, upgrade/rollback, images и подпись macOS.

См. [актуальный инженерный план](https://github.com/KLORQ/quarshell-workspace/blob/main/plans/REMAINING.md),
[таблицу OpenShell compatibility](openshell-compatibility.md) и
[runtime acceptance matrix](runtime-acceptance.md). MXC и GPU/VFIO заблокированы
до появления совместимых устройств и runtime dependencies.
