<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Recreate a sandbox

Use recreate when policy, proxy bindings, LogConfig, or resource limits must
apply from a clean create. Running containers keep create-time HostConfig until
replaced.

## What delete removes

| Removed | Kept |
|---------|------|
| `quarshell-<name>` | Gateway provider secrets |
| `quarshell-proxy-<name>` | Gateway registry entries until deleted |
| `quarshell-net-<name>` | |
| `quarshell-data-<name>` volume | |
| `quarshell-ca-<name>` volume | |

Deleting the data volume clears Cursor `agent login` state.

## Procedure

```bash
quarshell sandbox delete cursor

quarshell gateway ensure
quarshell doctor
quarshell provider list

quarshell sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy quarshell-cli/policies/cursor-github-push-quarshell.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g

quarshell sandbox connect cursor -- agent login
quarshell logs cursor --tail --source proxy
```

## Hot reload vs recreate

| Change | Prefer |
|--------|--------|
| Narrow policy rule | `quarshell policy set … --wait` / rule approve |
| New memory / image / uploaded files | Recreate |
| Rotated secret in store | `provider refresh` (often enough) |
