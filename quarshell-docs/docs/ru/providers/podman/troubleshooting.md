<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Устранение неполадок (Podman)

| Симптом | Действие |
|---------|----------|
| `no API socket found` | `systemctl --user enable --now podman.socket`; задайте `CAUTEM_PODMAN_SOCKET` |
| macOS не коннектится | `podman machine start`; проверьте sock под `~/.local/share/containers/podman/` |
| Драйвер всё ещё docker | `export CAUTEM_DRIVER=podman` в том же shell, что CLI |
| Сеть sidecar сломана | Тест без proxy; netavark/CNI; см. [ограничения](./limitations.md) |
| Образа нет | Образ должен быть в store Podman (`podman images`), не только в Docker |

```bash
export CAUTEM_DRIVER=podman
podman info
cautem status
cautem doctor
```
