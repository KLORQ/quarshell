<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Logging (Docker)

## Container logs

Sandbox and proxy containers use Docker `json-file` with rotation:

| Setting | Default |
|---------|---------|
| Driver | `json-file` |
| `max-size` | `10m` |
| `max-file` | `3` |

```bash
export QUARSHELL_DOCKER_LOG_DRIVER=json-file   # or none
export QUARSHELL_DOCKER_LOG_MAX_SIZE=10m
export QUARSHELL_DOCKER_LOG_MAX_FILE=3
```

`none` disables Engine container logs when you rely only on gateway observation.

Daemon-wide fallback (all containers on the host):

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
```

## Observation ring

Gateway keeps a per-sandbox in-memory ring (4096 lines, 1 MiB). On Gateway-managed
sandbox deletion it archives the recent tail in the gateway data directory:
up to 1 MiB for each of the latest 64 deleted sandboxes, retained for up to 7
days. Archived logs remain workspace-scoped and can be read without `--tail`;
they cannot be followed live. A new sandbox that reuses the name clears the
older archive. Stream live logs with:

```bash
quarshell logs demo --tail --source proxy
```

Process logs (CLI / gateway / proxy) use slogx.
