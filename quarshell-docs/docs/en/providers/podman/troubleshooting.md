<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Troubleshooting (Podman)

| Symptom | Action |
|---------|--------|
| `no API socket found` | `systemctl --user enable --now podman.socket`; set `CAUTEM_PODMAN_SOCKET` |
| macOS connect fails | `podman machine start`; confirm machine sock under `~/.local/share/containers/podman/` |
| Driver still docker | Export `CAUTEM_DRIVER=podman` in the same shell as the CLI |
| Sidecar networking broken | Test without proxy; check netavark/CNI; see [limitations](./limitations.md) |
| Image not visible | Ensure the image is in the Podman store (`podman images`), not only Docker |

```bash
export CAUTEM_DRIVER=podman
podman info
cautem status
cautem doctor
```
