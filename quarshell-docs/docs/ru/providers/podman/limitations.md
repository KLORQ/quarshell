<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Ограничения Podman

| Область | Заметки |
|---------|---------|
| API | Только Engine API — без Libpod-native клиента в MVP |
| Rootless-сеть | Dual-homed proxy sidecar может потребовать настройки CNI/netavark |
| Desktop | На macOS нужен Podman Machine; пути сокетов отличаются от Linux |
| GPU CDI | Тот же путь DeviceRequests, что у Docker; проверьте на целевом хосте |
| Packaging | Compose-примеры заточены под Docker; для Podman адаптируйте mount сокета |

Сначала `create` → `exec` → `rm` без proxy, затем gateway и sidecar.
