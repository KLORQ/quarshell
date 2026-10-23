# Quarshell

Quarshell runs coding agents in policy controlled sandboxes. The `quarshell` CLI
starts agents in isolated Docker or Podman environments, limits workspace and
network access, and keeps configured provider credentials outside the
sandbox. Gateway, VM, SDK, policy, and runtime components are maintained as 10
independent Go modules. The UI is a separate TypeScript/React frontend; Gateway
is its Go backend.

The user guide is available in [English](https://klorq.github.io/sandbox.dev/)
and [Russian](https://klorq.github.io/sandbox.dev/ru/). It covers installation,
sandbox creation, network policy, provider profiles, security boundaries, and
current backend support.

## Quick start

Follow the [installation guide](https://klorq.github.io/sandbox.dev/get-started/)
and [first sandbox walkthrough](https://klorq.github.io/sandbox.dev/get-started/first-sandbox/).

## Independent repositories

Product components live in independent Git repositories. Go module APIs are
pre-stable; new module paths start at `v0.1.0`, while paths with existing public
tags continue at the next available `0.x` version.

| Area | Repositories |
| --- | --- |
| CLI and APIs | [CLI](https://github.com/KLORQ/quarshell-cli), [Go SDK](https://github.com/KLORQ/quarshell-sdk), [Python SDK](https://github.com/KLORQ/quarshell-python), [UI](https://github.com/KLORQ/quarshell-ui) |
| Policy and runtime | [Core](https://github.com/KLORQ/quarshell-core), [Policy](https://github.com/KLORQ/quarshell-policy), [Proxy](https://github.com/KLORQ/quarshell-proxy), [Runtime](https://github.com/KLORQ/quarshell-runtime) |
| Compute | [Driver](https://github.com/KLORQ/quarshell-driver), [VM driver](https://github.com/KLORQ/quarshell-driver-vm) |
| Gateway and contracts | [Gateway](https://github.com/KLORQ/quarshell-gateway), [Protocol](https://github.com/KLORQ/quarshell-proto), [SDK](https://github.com/KLORQ/quarshell-sdk) |
| Project resources | [Documentation](https://github.com/KLORQ/sandbox.dev), [Workspace and plans](https://github.com/KLORQ/quarshell-workspace), [Deployment](https://github.com/KLORQ/quarshell-deploy) |

## Workspace development

The [workspace repository](https://github.com/KLORQ/quarshell-workspace) holds
cross-module tooling and plans. Clone its pinned module versions with
`tools/checkout-modules.sh`, then run `task workspace:sync` and `task test`.
Individual module READMEs document module specific commands.

The [architecture consolidation plan](plans/ARCHITECTURE-CONSOLIDATION.md)
records the consolidation from 20 to 10 Go modules, records the
required compatibility gates, and tracks the migration of workspace tools
from Python to Go.

Product modules use Apache-2.0. The logging package retains its MIT license in `quarshell-core/slogx/LICENSE`.
