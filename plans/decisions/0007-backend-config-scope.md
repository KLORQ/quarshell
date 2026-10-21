# ADR-0007: Backend config schema vs backend behavior

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

M2 требует typed config каждого backend. Pinned upstream structs Kubernetes и
VM содержат настройки, поведение которых относится к M6: workspace modes,
managed SSH ingress, GPU/VFIO, proxy material, external supervisor binaries.
Если добавить такие поля как decode-only, конфигурация молча принимается и не
имеет эффекта. Это хуже явного отказа.

## Решение

- M2 владеет schema contract: каждый pinned backend key либо декодируется и
  валидируется, либо отклоняется как unknown setting с ошибкой старта.
- M2 не добавляет поля без поведения. Реализованная схема и fail-closed отказ
  считаются выполнением typed-config требования.
- M6 владеет поведением отклонённых ключей и переводит их в implemented только
  вместе с behavior-тестами.
- `task compat:backends` показывает implemented и rejected keys; silent no-op
  запрещён.

## Последствия

- Нет конфигурации, которая выглядит принятой, но ничего не делает.
- K8s/VM gaps видимы и принадлежат M6.
- Docker/Podman и gateway TOML имеют полный schema audit без legacy-полей.
