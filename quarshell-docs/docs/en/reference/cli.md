<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# CLI surface

```text
quarshell sandbox create|list|get|stop|start|delete|exec|connect|upload|download|…
quarshell cp <local-path> <sandbox>:/path | <sandbox>:/path <local-path>
quarshell sandbox template create|list|get|delete
quarshell sandbox provider …
quarshell provider create|list|get|refresh|update|delete|profile …
quarshell profile …
quarshell gateway add|select|ensure|info|login|…
quarshell policy get|set|…
quarshell rule get|approve|reject|…
quarshell workspace|forward|service|settings|whoami|logs|term
quarshell health|init|doctor|status|version|install|completions|ssh-proxy
```

`--gateway-endpoint URL` selects a Gateway directly, without resolving a
registered Gateway name. It can appear before or after the command path; the
selected endpoint takes precedence over the stored Gateway selection.
`--gateway-insecure` skips certificate verification for Gateway TLS and should
be used only for endpoints whose certificate cannot be trusted locally.
Repeat `-v` or `--verbose` to enable debug logging.
`--color auto|always|never` controls ANSI styling (`QUARSHELL_COLOR` sets the
default; in `auto` mode non-empty `NO_COLOR` disables styling and non-empty
`FORCE_COLOR` enables it when `NO_COLOR` is unset.

`quarshell forward service --target-port PORT [--target-host HOST] [--local
[BIND:]PORT] [SANDBOX]` opens a local TCP listener to a sandbox loopback
service. The target host defaults to `127.0.0.1`; `--local 0` requests an
OS-assigned local port.

`quarshell provider list [--page-size N] [--page-token TOKEN]
[--label-selector EXPR] [--all-workspaces] [-o table|yaml|json]` returns one
Gateway page. Pass `next_page_token` as `--page-token`; all-workspace listing
requires a platform administrator. Provider credentials remain redacted.
`quarshell provider list-profiles [--workspace NAME|--global] [--page-size N]
[--page-token TOKEN] [-o table|yaml|json]` pages through the selected profile
catalog; `provider profile list` accepts the same options.

`quarshell term --theme auto|dark|light` selects the TUI accent palette;
`QUARSHELL_THEME` sets the default. `auto` uses `COLORFGBG` when available and
falls back to dark.

## Create (compute)

| Flag | Meaning |
|------|---------|
| `--name` | Sandbox name |
| `--from` / `--image` | Image alias or OCI reference |
| `--gateway-workspace` | OpenShell Gateway workspace |
| `--policy` | OpenShell SandboxPolicy YAML |
| `--cpu` / `--memory` | Portable workload resource limits |
| `--template` | Named workload template |
| `--provider` | Credential provider (repeatable) |
| `--expose PORT` | Register a sandbox HTTP/WebSocket service (included in Gateway create) |
| `--expose-authorization-mode strip|bearer-passthrough` | Choose how the service proxy handles application Bearer authorization |
| `--gpu` | NVIDIA CDI devices |

`quarshell sandbox create --help` lists the accepted create flags. Creation always
uses the selected Gateway and OpenShell `CreateSandbox` RPC. Local build,
display, bind-mount, and Engine-specific create flags are not accepted. Each
`sandbox` leaf has its own help route; run
`quarshell sandbox <leaf> --help` for its accepted arguments.

Use `--gateway-workspace NAME` to select the target workspace. Image based
creates accept CPU, memory, GPU count, environment, labels, provider attachments,
policy, restart policy and driver config JSON. A shared template owns its
workload settings and rejects inline overrides. `--from` also accepts a local
`.tar`, `.tar.gz`, or `.tgz` rootfs archive when the selected Gateway is on this
machine and its compute driver supports rootfs staging. A foreground command
runs through native interactive exec after provisioning; `--detach` leaves it
running in the Gateway. Use
`quarshell sandbox connect NAME --editor vscode|cursor` to open a remote IDE.

`--expose PORT` keeps the sandbox supervisor available. Gateway
creation sends the exposure with the sandbox request and asks the Gateway to
remove the new sandbox if service registration fails. This is a compensating
cleanup; a cleanup failure is reported for reconciliation.
`--expose-authorization-mode` defaults to `strip`; `bearer-passthrough` forwards
one syntactically valid Bearer header to the service.

Repeat `--forward PORT` during create to start a background local TCP forward
after provisioning. `quarshell forward start [BIND:]PORT SANDBOX` uses the native
OpenShell `ForwardTcp` relay to a sandbox loopback port; `--forward` uses the
same port on the local and sandbox sides.

The SSH helper invoked by generated `sandbox ssh-config` entries supports the
current gateway shorthand, a named gateway, and an explicit gateway URL with a
sandbox token. Run `quarshell ssh-proxy --help` for the exact forms.

Host bind mounts, local image builds, display, and local runtime toggles are not
part of the OpenShell Gateway create contract. `--upload` streams files through
the native OpenShell interactive execution RPC. A foreground TTY command is
started as the canonical main process in the create request and the CLI
attaches to its retained PTY. A foreground command without a TTY uses native
interactive exec after provisioning; detached commands are sent in the create
request.

Add `--gateway` to a `sandbox template` command to manage the template in the
selected Gateway workspace. Gateway template creation also accepts repeatable
`--annotation KEY=VALUE`, `--ready-within DURATION`, and `--max-burst COUNT`;
these options are rejected for local templates rather than ignored. Durations
use Go syntax such as `30s`, `5m`, or `1h`.

## Sandbox subcommands

| Command | Shape |
|---------|-------|
| `sandbox list` | `--page-size N`, `--page-token TOKEN`, `--selector EXPR`, `-o table\|yaml\|json`, `--ids`, `--names`, `--all-workspaces` |
| `sandbox get` | `NAME [--policy-only]` |
| `sandbox start` / `sandbox stop` | Optional `NAME` (defaults to the last-used sandbox) |
| `sandbox delete` | `NAME...` or `--all` (exclusive) |
| `sandbox upload` | `NAME LOCAL_PATH [SANDBOX_DEST] [--no-git-ignore]` |
| `sandbox download` | `NAME SANDBOX_PATH [LOCAL_DEST]` |
| `sandbox ssh-config` | `NAME [--install]` |
| `sandbox connect` | `[NAME] [--editor vscode\|cursor]` or `[NAME] -- COMMAND...` |
| `sandbox provider list` | `[SANDBOX] [--page-size N] [--page-token TOKEN] [-o table\|yaml\|json]` |
| `sandbox provider attach` / `detach` | `SANDBOX PROVIDER` (prints the change ID) |
| `sandbox provider status` | `SANDBOX PROVIDER [--receipt ID] [--wait] [--timeout SECONDS] [-o table\|yaml\|json]` |

`sandbox list` returns one page. Pass `next_page_token` back as `--page-token`;
table output prints the token to stderr, and JSON/YAML include it in the response.
`--all-workspaces` requires a platform administrator.
| `sandbox template create` | `NAME` with image/from/policy, CPU/memory/PIDs/GPU, env, labels, driver config and provider flags |
| `sandbox template list` | `--gateway [--page-size N] [--page-token TOKEN] [--label-selector EXPR] [-o table\|yaml\|json]` |
| `sandbox template get` / `delete` | `NAME [--gateway]` |

Gateway template listing returns one page. Pass `next_page_token` back as
`--page-token`; table prints it to stderr, while JSON/YAML include it.

`--editor` and a command cannot be combined. `--no-git-ignore` is upload-only;
the default upload destination is `/workspace` and the default download
destination is the current directory.

## Workspace commands

| Command | Shape |
|---------|-------|
| `workspace create` | `--name NAME`, repeatable `--label KEY=VALUE` |
| `workspace list` | `--page-size N`, `--page-token TOKEN`, `--label-selector KEY=VALUE[,KEY=VALUE...]`, `-o table\|yaml\|json` |
| `workspace member list` | `--workspace NAME`, `--page-size N`, `--page-token TOKEN`, `-o table\|yaml\|json` |
| `workspace get` | `NAME` |
| `workspace delete` | `NAME...` |

The list command returns one page. Pass its `next_page_token` as `--page-token`
to fetch the next page. Label selector pairs are combined with AND.
Workspace member listing also returns one page and accepts its continuation token.

## Settings

`quarshell settings get [SANDBOX] [--global] [--json]` reads effective settings
for the last-used sandbox or a named sandbox; `--global` reads Gateway-wide
settings and does not accept a sandbox name. `settings set [SANDBOX] --key
KEY --value VALUE` and `settings delete [SANDBOX] --key KEY` update or remove
a sandbox setting. Add `--global --yes` to set or delete a Gateway-wide
setting. Values are checked against OpenShell's registered keys and types.

## Service endpoints

Use `quarshell service expose SANDBOX PORT [NAME] [--authorization-mode strip|bearer-passthrough]`,
`service list [SANDBOX] [--page-size N] [--page-token TOKEN] [--all-workspaces] [-o table|yaml|json]`, and `service get|delete SANDBOX [NAME]`. Service listing returns one cursor page; table output prints its next token, while JSON/YAML include `next_page_token`. `--all-workspaces` requires a platform administrator and overrides the selected workspace. The current
gateway defaults to `strip`, which removes application Authorization headers
before proxying. `bearer-passthrough` forwards one syntactically valid Bearer
header unchanged to the service over HTTP and WebSocket; this mode requires a
Gateway connection. The service
table includes each endpoint URL; JSON/YAML also include endpoint ID, sandbox
and workspace metadata, domain status, and authorization mode.

## Terminal browser

`quarshell term` refreshes the sandbox inventory every two seconds. Use `↑`/`↓`
or `j`/`k` to select a sandbox, `/` to filter by name/status/image, and `d` to
view details. Press `enter` or `c` to connect, `e` for an interactive shell,
`s` to stop a running sandbox, or `S` to start a stopped sandbox. Press `x` and
confirm to delete. `l` follows the selected sandbox logs, `a` follows all logs,
`s` cycles the live log source filter (local: `all`, `sandbox`, `proxy`; Gateway:
`all`, `gateway`, `sandbox`), and `p`
pauses or resumes the live log display while the bounded buffer keeps
collecting. In the live log view, use `↑`/`↓` or `j`/`k` to select a line,
`Enter` to open its details, `PgUp`/`PgDn` or `Ctrl-B`/`Ctrl-F` to move by a viewport, and `g`/`G`
to select the first/last line. `L` prints the selected logs and returns, `r`
refreshes, and `q` exits. Press `y` to request copying the selected line/range or `Y`
to request copying visible log lines through the terminal clipboard; support
depends on the terminal. Press `v` to start or end a color-highlighted visual
range; move with `j`/`k`, then press `y` to copy that range. Press `Esc` to leave a detail or
live-log view.

## Soft defaults

When flags (and template) omit sizing, config/env may fill gaps — never a
hard-coded memory. See [Docker resources](../providers/docker/resources.md).
