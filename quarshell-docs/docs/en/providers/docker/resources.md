<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Resources and limits

Sandbox resource requests are sent to the selected Gateway with the native
OpenShell `CreateSandbox` request. The Docker driver and its configuration live
on the Gateway host; the CLI does not modify local Docker containers.

| CLI option | OpenShell request field |
|------------|-------------------------|
| `--cpu` | CPU resource request |
| `--memory` | Memory resource request |
| `--gpu [COUNT]` | GPU request |
| `--driver-config-json` | Driver-specific configuration |

The Gateway operator configures driver-wide limits such as the sandbox PID
limit in its `gateway.toml`. There is no per-sandbox `--pids-limit` option.
There is no hard-coded CLI memory default; set `--memory` or use a shared
workload template.

Example:

```bash
quarshell sandbox create --name worker --from cursor --cpu 2 --memory 4Gi --gpu 1
```
