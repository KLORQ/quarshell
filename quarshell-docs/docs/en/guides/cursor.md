<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Cursor agent

## Image

```bash
task docker:agent:cursor
# or: docker pull ghcr.io/klorq/quarshell/sandboxes/cursor:latest
```

## Providers and policy

```bash
CURSOR_API_KEY=… quarshell provider create --name cursor --type cursor --credential CURSOR_API_KEY
GITHUB_TOKEN=…   quarshell provider create --name gh --type github --credential GITHUB_TOKEN

ORG=YOUR_ORG
sed "s/YOUR_ORG/${ORG}/g" quarshell-cli/policies/cursor-github-push.yaml \
  > /tmp/cursor-github-push.yaml
```

The `cursor` profile primarily contributes egress allow rules
(`**.cursor.sh` / `**.cursor.com`). It does **not** inject `CURSOR_API_KEY` into
the guest (`inject_env: false`).

Builtin `github` is mostly read/clone. Push needs a write-capable base policy
(example above).

## Create

```bash
quarshell sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /tmp/cursor-github-push.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

Verify:

```bash
quarshell sandbox exec cursor -- env | grep -E 'TOKEN|KEY|CURSOR' || true
# expect: GITHUB_TOKEN=quarshell:resolve:… ; no CURSOR_API_KEY
```

## Login and run

```bash
quarshell sandbox connect cursor -- agent login
quarshell sandbox connect cursor -- agent
```

OAuth state lands under the persist volume (`/quarshell/data`). After
`stop`/`start`, login usually survives until the volume is deleted.

```bash
quarshell logs cursor --tail --source proxy
quarshell term
```

In the TUI, use the arrow keys or `j`/`k` to select a sandbox, `enter`/`c` to
connect, `e` to exec, `l` to follow its logs, and `a` to follow logs for all
sandboxes. In live logs, `p` pauses or resumes display while the bounded buffer
keeps collecting, `s` cycles the source filters for the active connection, and
`r` reconnects. Local logs offer `all`, `sandbox`, and `proxy`; Gateway logs
offer `all`, `gateway`, and `sandbox`. Press `Esc` to return from logs and `q`
to quit. Use `↑`/`↓` or `j`/`k` to select a line, `Enter` to show its details,
and `g`/`G` to select the first/last line.
Press `y` to copy the selected line or `Y` to copy the visible log lines through
the terminal clipboard; support depends on the terminal. Press `v` to start or
end a color-highlighted visual range, move with `j`/`k`, and press `y` to copy
the range.

## Host IDE (C2)

The uploaded checkout lives under `/workspace`; the host working directory is
not mounted into the sandbox. Run network and git commands through the sandbox:

```bash
quarshell sandbox exec cursor -- go test ./...
quarshell sandbox exec cursor -- gh repo view
```

Host directories are not mounted into Gateway sandboxes. Use provider profiles
for credentials and upload only the files the workload needs.

## Modes

| Mode | Meaning |
|------|---------|
| C1 | Agent CLI inside the sandbox |
| C2 | Host IDE + sandboxed workspace / exec |
