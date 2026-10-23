<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Your first sandbox

With the CLI installed and the gateway running, create a sandbox for the
project you want the agent to work on.

Set `QUARSHELL_USE_GHCR=1` if you want the `base` alias to use the published
Quarshell image, and save a policy file as shown in the [macOS](run-macos.md)
or [Linux](run-linux.md) walkthrough. The commands below assume that file is
`~/.config/quarshell/default-policy.yaml`.

## Create

Create a Gateway managed sandbox:

```bash
quarshell sandbox create \
  --name demo \
  --gateway-workspace default \
  --from base \
  --policy ~/.config/quarshell/default-policy.yaml \
  --upload "$PWD" \
  --memory 2g
```

| Flag | What it does |
|------|--------------|
| `--name` | Sandbox name |
| `--gateway-workspace` | Workspace on the selected Gateway |
| `--from` | Image alias; `base` uses GHCR when `QUARSHELL_USE_GHCR=1` |
| `--policy` | Sandbox policy YAML file |
| `--upload` | Copy a local project directory into `/workspace` |
| `--memory` | Sandbox memory limit |

## Work inside

```bash
quarshell sandbox list
quarshell sandbox exec demo -- uname -a
quarshell sandbox connect demo
```

`connect` opens an interactive shell inside the sandbox. A Gateway workspace
selects resource scope and does not mount a host folder.

## Give it credentials

Optional. Secrets live in the gateway and never enter the sandbox — the agent
only sees placeholders:

```bash
GITHUB_TOKEN=… quarshell provider create --name gh --type github --credential GITHUB_TOKEN

quarshell sandbox create \
  --name demo-with-gh \
  --gateway-workspace default \
  --from base \
  --policy ~/.config/quarshell/default-policy.yaml \
  --provider gh
```

More: [Credential providers](../guides/credentials.md) ·
[Cursor agent](../guides/cursor.md).

## Watch, stop, clean up

```bash
quarshell logs demo --tail --source proxy
quarshell sandbox stop demo
quarshell sandbox delete demo
```

`delete` removes the sandbox, its proxy sidecar, network and data volumes.
Provider secrets stay in the gateway.

## What's next

When the agent hits a blocked host, it gets a 403 with the reason. See
[Policy](../guides/policy.md) to approve a narrow rule without recreating the
sandbox, and [Recreate a sandbox](../guides/recreate-sandbox.md) for changes
that do need a fresh container.
