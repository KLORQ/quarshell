<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Create a sandbox (Podman-backed Gateway)

```bash
cautem sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g

cautem sandbox exec demo -- uname -a
cautem sandbox delete demo
```

With proxy and providers, ensure the gateway is selected first:

```bash
cautem gateway select local
cautem sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

Configure Podman on the Gateway host. `CAUTEM_DRIVER=podman` on the CLI does
not select the remote Gateway compute driver. Network policy is enforced by the
Gateway sandbox runtime.
