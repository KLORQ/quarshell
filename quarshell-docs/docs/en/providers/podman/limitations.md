<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Podman limitations

| Area | Notes |
|------|-------|
| API | Engine API only — no Libpod-native client in MVP |
| Rootless networking | Dual-homed proxy sidecar may need CNI/netavark tuning |
| Desktop | Podman Machine required on macOS; socket paths differ from Linux |
| GPU CDI | Same DeviceRequests path as Docker; validate on target host |
| Packaging | Compose examples target Docker; adapt socket mounts for Podman |

Prefer validating `create` → `exec` → `rm` without proxy, then enable gateway
and sidecar.
