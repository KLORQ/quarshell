<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Create a sandbox (Podman-backed Gateway)

```bash
quarshell sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g

quarshell sandbox exec demo -- uname -a
quarshell sandbox delete demo
```

With proxy and providers, ensure the gateway is selected first:

```bash
quarshell gateway select local
quarshell sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

Configure Podman on the Gateway host. `QUARSHELL_DRIVER=podman` on the CLI does
not select the remote Gateway compute driver. Network policy is enforced by the
Gateway sandbox runtime.
