<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Policy

Sandboxes start default-deny. The base policy YAML plus composed credential
provider profiles form the effective allowlist enforced by the egress proxy.

## Set and get

```bash
quarshell policy get demo --full
quarshell policy set demo --policy /path/to/policy.yaml --wait
```

`--wait` blocks until the sidecar reloads the bind-mounted effective YAML.

Preview an incremental network policy change before applying it:

```bash
quarshell policy update demo --add-allow 'api.github.com:443,8443:GET:/repos/**' --rule-name github --binary /usr/bin/agent --dry-run
```

`--dry-run` reads the current base policy, prints the merged YAML, and does not
send an update to the Gateway.

Removal-only updates use the Gateway's atomic merge API:

```bash
quarshell policy update demo --remove-endpoint api.github.com:443
quarshell policy update demo --remove-rule github
quarshell policy update demo --add-endpoint api.example.com:443:read-only:rest --rule-name example-api --remove-rule old-api
```

Simple `--add-endpoint`, L7 additions, and removals can be combined atomically.
L7 additions require `--rule-name` and the complete rule binary scope via
repeatable `--binary` flags, or `--any-binary` for a rule that allows all
binaries. The endpoint spec is `host:port[,port...]:METHOD:path_glob`; every
port on the selected endpoint must be listed. `--endpoint-path` selects an
endpoint with that policy path (an empty value selects an endpoint without a
path selector). `--dry-run` previews the merged result without updating the
Gateway.
`--timeout SECONDS` sets the policy apply deadline (default: 60).

## policy.local (guest)

Inside the sandbox, agents can query denials and propose narrow rules via
`policy.local` (MITM HTTPS to the sidecar):

| Method | Path |
|--------|------|
| GET | `/v1/policy/current` |
| GET | `/v1/denials?last=N` |
| POST | `/v1/proposals` |
| GET | `/v1/proposals/{id}/wait` |

## Operator approve loop

```bash
quarshell rule get --status pending
quarshell rule approve --chunk-id chk_…
# or: quarshell rule reject --chunk-id chk_… --reason "narrow to /docs"
```

Approve merges into the sandbox base policy, rewrites the live policy file, and
hot-reloads the sidecar. Prefer the smallest `addRule` that unblocks the task.

## Schema keys

Policy documents use `filesystem_policy`, `landlock`, and `network_policies`
(`quarshell-policy/corepolicy`). L7 rules may set `protocol: rest|graphql|mcp` with
method/path/tool constraints.

## Built-in request redaction

The pinned `openshell/regex` middleware can redact OpenAI-style API keys in
HTTP request bodies and client-to-upstream WebSocket text messages:

```yaml
network_middlewares:
  redact-provider-keys:
    middleware: openshell/regex
    order: 10
    config:
      mode: redact
    on_error: fail_closed
    endpoints:
      include: ["api.example.com"]
```

`mode` may be omitted or set to `redact`; other fields and modes are rejected.
The fixed pattern is `sk-[A-Za-z0-9_-]{16,}` and matches become
`[REDACTED]`. Bodies/messages must be UTF-8 and at most 256 KiB. Binary and
server-to-client WebSocket messages are never transformed; client binary
messages and invalid/oversized input follow `on_error` (default
`fail_closed`). Findings record only type, severity, confidence, and match
count. The original key and request body are not included in audit output.

## Related

- Skill: `/etc/quarshell/skills/policy-advisor`
