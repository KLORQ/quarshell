# ADR-0004: Runtime security без Rust fallback

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

Текущий runtime использует Docker defaults, no-new-privileges и NET_RAW drop,
но не реализует upstream isolation boundary: trusted binary identity, seccomp
notification broker, FD injection и Landlock verification.

## Решение

- Новый Go launcher/broker реализует Isolation boundary.
- Проверяются capabilities, process identity, no_new_privs, namespaces/cgroups,
  Landlock, seccomp notification, FD lifecycle и network mediation.
- Каждый обязательный шаг проверяется до запуска workload.
- `LockOSThread` не считается доказательством однопоточности; thread scope
  Landlock/seccomp проверяется на выбранной библиотеке и kernel ABI.
- Отсутствие обязательной capability — отказ запуска, не best-effort.
- Production не вызывает Rust OpenShell binaries. Upstream допустим только как
  conformance oracle в отдельном runner.
- Если Go-реализация не обеспечивает гарантию, milestone остаётся открытым.

## Последствия

- M3 — высокий технический риск и отдельный Linux host lane.
- Security evidence привязан к instance, generation и host capability set.
- Нельзя объявить полный перенос, оставив старый sandbox в production.
