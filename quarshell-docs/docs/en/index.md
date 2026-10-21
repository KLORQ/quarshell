---
template: home.html
title: cautem Sandbox — Sandboxes for coding agents
description: >-
  Run coding agents in Docker or Podman sandboxes with policy-controlled network
  access and credentials kept outside the sandbox.
hide:
  - navigation
  - toc
hero:
  eyebrow: CONTROL THE AGENT'S REACH
  title: Sandboxes for coding agents
  text: >-
    Run Cursor, Claude, Codex or any agent you bring in a container that sees
    your project and reaches only the hosts you allow. Provider credentials
    stay outside the sandbox.
  flow_label: Codex sandbox workflow preview
  tab_create: Create
  tab_run: Run Codex
  tab_observe: Monitor
  flow_note: After importing the codex-api profile and configuring the codex credential provider. --from codex selects the agent image; --provider codex attaches credentials. Status lines are illustrative; cautem term opens the TUI.
  primary:
    label: Get started
    link: get-started/
  secondary:
    label: How it works
    link: "#how-it-works"
---

<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

## Why

Coding agents run shell commands, install packages and call APIs for you. Run
them straight on your machine and they get your whole home directory, SSH
keys, cloud credentials and an open internet connection. One bad prompt or
poisoned package is enough to leak or break something.

cautem runs the agent in a container with a restricted workspace and
**policy-controlled network access**. Credentials managed through providers
stay outside the container.

<div class="grid cards" markdown>

-   :material-folder-lock:{ .lg .middle } __Only the project__

    ---

    Your folder is mounted at `/workspace`. No `docker.sock`, no `~/.ssh`,
    no `~/.aws` inside.

-   :material-wall-fire:{ .lg .middle } __Default-deny network__

    ---

    Nothing leaves the sandbox unless the policy allows the host — and for
    HTTPS, the method and path.

-   :material-key-chain:{ .lg .middle } __Secrets stay outside__

    ---

    For configured providers, the agent sees placeholders. The proxy inserts
    real tokens only into allowed requests.

-   :material-sync:{ .lg .middle } __Live policy__

    ---

    The agent proposes a narrow rule, you approve it, the proxy reloads in
    about a second — no rebuild.

-   :material-robot-outline:{ .lg .middle } __Any agent__

    ---

    Ready images for Cursor, Claude and Codex, or bring your own container.

-   :material-docker:{ .lg .middle } __Docker or Podman__

    ---

    Same layout on both engines; Kubernetes and MicroVM are on the way.

</div>

## How it works { #how-it-works }

```mermaid
sequenceDiagram
    autonumber
    actor You
    participant S as Sandbox (agent)
    participant P as Egress proxy
    participant U as Internet
    You->>S: cautem sandbox create
    S->>P: HTTPS request with a secret placeholder
    P->>P: Check host, method and path against the policy
    alt allowed
        P->>U: Forward with the real secret
        U-->>S: Response
    else blocked
        P-->>S: 403 + reason
        S->>You: Propose a narrow rule
        You->>P: cautem rule approve
    end
```

1. **Create.** `cautem sandbox create` starts two containers on a private
   network: the sandbox with the agent and your workspace, and an egress proxy
   next to it.
2. **Every request goes through the proxy.** The sandbox has no other way out,
   so all traffic is checked against your policy.
3. **Secrets are swapped on the way out.** The agent only ever holds
   placeholders like `cautem:resolve:env:GITHUB_TOKEN`; the proxy puts the
   real token into requests the policy allows.
4. **Blocked means explained.** A denied request gets a 403 with the reason, so
   the agent can ask for exactly the access it needs.
5. **You stay in control.** Approve or reject the proposal; the new policy is
   live in about a second, without recreating the sandbox.

More detail: [Architecture](concepts/architecture.md) · [Security](concepts/security.md).
