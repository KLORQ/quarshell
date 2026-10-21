---
status: soon
---

<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Kubernetes

<div class="ws-soon" markdown>

![Under construction](../../assets/images/under-construction.svg){ width="420" }

**Coming soon.** This provider is under construction.

</div>

## What is planned

- Sandboxes run as pods, selected with `CAUTEM_DRIVER=kubernetes`.
- The gateway ships as a Helm chart: persistent storage, ServiceAccount,
  NetworkPolicy, Ingress and optional OIDC.
- The same policy, proxy and credential providers as on Docker.

Today the `kubernetes` driver is a stub. Use [Docker](docker/index.md) or
[Podman](podman/index.md) in the meantime.
