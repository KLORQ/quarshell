<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Create a sandbox (Docker-backed Gateway)

## Minimal create

```bash
cautem sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g
```

With an agent image and credential providers:

```bash
cautem sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

## Lifecycle

```bash
cautem sandbox list
cautem sandbox status demo
cautem sandbox exec demo -- uname -a
cautem sandbox connect demo          # connect to the Gateway sandbox
cautem sandbox stop demo
cautem sandbox start demo
cautem sandbox delete demo
```

Lifecycle commands operate on the selected Gateway sandbox. Configure Docker on
the Gateway host; the CLI does not create local Docker containers.

## What create does

1. Resolves the image or shared workload template.
2. Sends policy, providers, resource limits, labels, environment, and command to
   the Gateway `CreateSandbox` RPC.
3. Uploads requested files over the native sandbox exec stream.
4. Starts a foreground command with native interactive exec, or leaves a
   detached command running in the Gateway.

## Templates

Bake sizing into a reusable template (OpenShell-style):

```bash
cautem sandbox template create \
  --name desk \
  --from cursor \
  --memory 2g \
  --cpu 2 \

cautem sandbox create --template desk --name worker --upload "$PWD"
```

Flag values override template fields. Soft defaults from config/env fill gaps
only when flags and template omit them — see [Resources](./resources.md).

Templates are local to the CLI profile by default. Add `--gateway` to manage a
workspace-scoped template through the selected Gateway:

```bash
cautem sandbox template create shared-agent --gateway \
  --image ghcr.io/example/agent:stable --cpu 500m --memory 2Gi \
  --gpu 1 --env MODE=fast --label team=runtime
cautem sandbox template list --gateway
cautem sandbox template get shared-agent --gateway
cautem sandbox template delete shared-agent --gateway
```

Shared templates are stored by the Gateway. `sandbox create --template` uses
the selected Gateway workspace.
