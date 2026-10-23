<h1 align="center">Quarshell Python SDK</h1>

<p align="center">
  <strong>Python SDK for quarshell-gateway</strong><br>
  Manage sandboxes over gRPC and execute through OpenShell RPC.
</p>

<p align="center">
  <a href="https://www.apache.org/licenses/LICENSE-2.0"><img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"></a>
</p>

<p align="center">
  <sub>Part of the <a href="https://github.com/KLORQ">quarshell</a> ecosystem</sub>
</p>

---

## Overview

The [gateway guide](https://klorq.github.io/sandbox.dev/guides/gateway/) describes the server this client calls. For OpenShell parity, see [compatibility status](https://klorq.github.io/sandbox.dev/reference/openshell-compatibility/).

The SDK uses the pinned OpenShell v0.1.4 gRPC contract for Gateway operations.
HTTP remains for the transport health probe. SSH session tokens can be created
and revoked through the SDK; interactive SSH terminals are managed by the CLI.

---

## Installation

```bash
pip install -e .
```

Run the command in this checkout. The `quarshell` package is not published on PyPI yet.

---

## Quick Start

```python
import os

from quarshell import Client, Provider, Sandbox, WorkloadTemplate

with Client("http://127.0.0.1:7443") as c:
    c.create(Sandbox(name="demo", image="python:3.12-slim"))
    c.stop("demo")
    c.start("demo")
    print(c.list())
    print(c.exec("demo", "echo", "hi"))

    template = c.create_template(
        WorkloadTemplate(
            name="worker",
            image="python:3.12-slim",
            cpu="500m",
            memory="1Gi",
            environment={"MODE": "worker"},
        )
    )
    print(c.list_templates())
    print(c.get_template(template.name))
    c.delete_template(template.name)

    c.put_provider(
        Provider(
            name="model-api",
            type="openai",
            env_vars=["API_KEY"],
            credentials={"API_KEY": os.environ["MODEL_API_KEY"]},
            config={"base_url": "https://api.example.test"},
        )
    )
    c.attach_provider("demo", "model-api")
    print(c.effective_policy("demo").decode())
    for proposal in c.list_proposals(sandbox="demo", status="pending"):
        print(proposal.id, proposal.intent_summary)
```

`Sandbox` creation, listing, lookup, start, stop, and deletion use the pinned
OpenShell Gateway RPCs. Creation supports CPU/memory/GPU limits, environment,
TTY, workload templates, driver configuration, and native service exposures.
Native lifecycle RPCs use request IDs for idempotency and return OpenShell
sandbox resources directly; query exposed service URLs with `list_services`.

```python
from quarshell import Client, Sandbox, ServiceExposure

with Client("https://gateway.example") as client:
    sandbox = client.create(
        Sandbox(
            name="web",
            image="nginx:stable",
            service_exposures=[ServiceExposure(name="web", target_port=80)],
        )
    )
    print(sandbox.name)
```

Interactive execution streams stdin and separate output channels. Input frames
can resize a TTY while the process is running:

```python
from quarshell import Client, ExecInput

with Client("https://gateway.example") as client:
    for event in client.exec_interactive(
        "worker",
        "bash",
        "-i",
        inputs=[ExecInput.send(b"echo hello\n"), ExecInput.resize(120, 40)],
        tty=True,
    ):
        if event.stdout:
            print(event.stdout.decode(), end="")
        if event.stderr:
            print(event.stderr.decode(), end="")
        if event.exit_code is not None:
            print(f"exit: {event.exit_code}")
```

Set `QUARSHELL_GATEWAY_TOKEN` or pass `token="…"` to `Client` when the
gateway requires bearer authentication. RPC failures are raised as `grpc.RpcError`;
the health probe uses `httpx` exceptions.

Call `whoami()` to read the authenticated subject, roles, scopes, and identity
provider reported by the gateway.

Platform administrators can page through the Gateway audit event stream with
`list_audit_events` and its `next_after_id` marker.

Create an SSH session token for a sandbox and revoke it when finished. The
token is omitted from the session object's representation because it is secret:

```python
with Client("https://gateway.example") as client:
    session = client.create_ssh_session("worker")
    print(session.gateway_host, session.gateway_port, session.expires_at_ms)
    client.revoke_ssh_session(session.token)
```

Shared workload templates are scoped to `Client.workspace`. Use
`WorkloadTemplate(workspace="...")` or pass `workspace="..."` to get/list/delete
to select another workspace. Template listing follows the Gateway cursor until
all pages are read and fails if the Gateway repeats a cursor.

Provider credentials are write-only: provider reads return credential key names
in `env_vars` and never return secret values. Use
`update_provider_credentials(name, credentials)` to patch secrets without
replacing existing provider metadata; refresh material and credential maps are
excluded from the public dataclass representation.

Policy updates use the current resource version/revision before writing. Proposal
approval and rejection verify the requested sandbox before applying the decision.
Provider profiles support `list_profiles`, `get_profile`, `create_profile`,
`put_profile`, and `delete_profile`; pass `scope="workspace"` with a workspace
name for workspace-scoped profiles. Updates require the resource version returned
by `get_profile` and accept documents up to 1 MiB.

Workspace administration is available through `list_workspaces`,
`get_workspace`, `create_workspace`, and `delete_workspace`. `get_workspace`
also reads every member page. Use `add_workspace_member` and
`remove_workspace_member` to manage member roles.

The `list_*_page` methods return a `CursorPage` with `items` and
`next_page_token` for templates, providers, attached sandbox providers,
provider profiles, workspaces, workspace members, services, and sandboxes
(`list_page`). Pass the token to the next call to continue without collecting
the full result set.

```python
page = c.list_page(page_size=100)
while True:
    for sandbox in page.items:
        print(sandbox.name)
    if not page.next_page_token:
        break
    page = c.list_page(page_size=100, page_token=page.next_page_token)
```

Policy history also exposes `list_policy_revisions_page` and
`list_policy_revision_records`, returning Gateway revision status, timestamps,
policy hash, and provenance. The existing `list_policy_revisions` result remains
available for compatibility with its exact YAML byte counts.
Platform administrators can use `list_global_policy_revisions_page` and
`list_global_policy_revision_records` for global history.

Platform administrators can also pass `all_workspaces=True` to provider and
service page or all-pages list methods. A workspace argument cannot be combined
with global provider listing.

Manage sandbox service endpoints with `expose_service`, `get_service`,
`list_services`, and `delete_service`. Endpoints route to a sandbox port through
the gateway relay; custom backend hosts are not part of this API. Pass
`authorization_mode="strip"` (the default) to remove application Bearer
headers, or `authorization_mode="bearer-passthrough"` to forward one valid
Bearer header to the service over HTTP and WebSocket.

The client also exposes typed global setting access through `get_setting`, `list_settings`, and `put_setting`.

Sandbox log reads and pushes use OpenShell `GetSandboxLogs` and
`PushSandboxLogs` directly. `watch_sandbox` and `watch_sandbox_logs` stream
native `WatchSandbox` events with optional cursor resumption. The pinned API has
no workspace-wide sandbox inventory watch.

Large custom root filesystems can be staged without buffering them in memory:

```python
with open("rootfs.tar", "rb") as source, Client("http://127.0.0.1:7443") as client:
    source.seek(0, 2)
    size_bytes = source.tell()
    source.seek(0)
    token = client.upload_rootfs_tar("rootfs.tar", size_bytes, source)
```

The returned staging token belongs in the selected driver's
`rootfs_tar_staging_token` configuration for sandbox creation. The Python
process must share the gateway filesystem, as required by OpenShell's local
staging path contract.

Generated protocol clients are checked in. From this module in the
multi-repository workspace, regenerate the first-party API from
`quarshell-proto/proto` with `python scripts/generate_proto.py --control-only`.
Run `python scripts/generate_proto.py` to also regenerate OpenShell clients from
the checked-in v0.1.4 source lock; generation verifies every pinned proto file
digest before writing output.

The wheel includes partial PEP 561 typing metadata and generated protocol stubs.

---

## Related

| Resource | Link |
|----------|------|
| Organization | [https://github.com/KLORQ](https://github.com/KLORQ) |
| Go SDK | [quarshell/quarshell-sdk](https://github.com/KLORQ/quarshell-sdk) |
| Gateway | [quarshell/quarshell-gateway](https://github.com/KLORQ/quarshell-gateway) |

## License

Apache-2.0 © quarshell
