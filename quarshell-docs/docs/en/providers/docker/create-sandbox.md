<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Create a sandbox (Docker-backed Gateway)

## Minimal create

```bash
quarshell sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g
```

With an agent image and credential providers:

```bash
quarshell sandbox create \
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
quarshell sandbox list
quarshell sandbox status demo
quarshell sandbox exec demo -- uname -a
quarshell sandbox connect demo          # connect to the Gateway sandbox
quarshell sandbox stop demo
quarshell sandbox start demo
quarshell sandbox delete demo
```

Lifecycle commands operate on the selected Gateway sandbox. Configure Docker on
the Gateway host; the CLI does not create local Docker containers.

## What create does

1. Resolves a matching local CLI template first; if none exists, it uses the
   shared workload template in the selected Gateway workspace.
2. Sends policy, providers, resource limits, labels, environment, and command to
   the Gateway `CreateSandbox` RPC.
3. Uploads requested files over the native sandbox exec stream.
4. Starts a foreground command with native interactive exec, or leaves a
   detached command running in the Gateway.

## Templates

Bake sizing into a reusable template (OpenShell-style):

```bash
quarshell sandbox template create \
  --name desk \
  --from cursor \
  --memory 2g \
  --cpu 2 \

quarshell sandbox create --template desk --name worker --upload "$PWD"
```

The template supplies workload defaults. Create flags cannot override its
image, source, CPU, memory, GPU, environment, or driver config. Policy,
providers, labels, and forwards can still be supplied when creating a sandbox.
See [Resources](./resources.md).

Templates are local to the CLI profile by default. Add `--gateway` to manage a
workspace-scoped template through the selected Gateway:

```bash
quarshell sandbox template create shared-agent --gateway \
  --image ghcr.io/example/agent:stable --cpu 500m --memory 2Gi \
  --gpu 1 --env MODE=fast --label team=runtime
quarshell sandbox template list --gateway
quarshell sandbox template get shared-agent --gateway
quarshell sandbox template delete shared-agent --gateway
```

Shared templates are stored by the Gateway. When no local template has that
name, `sandbox create --template` resolves it in the selected Gateway workspace.
