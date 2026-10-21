<h1 align="center">quarshell-proxy</h1>

<p align="center">
  <strong>Egress proxy for sandboxes</strong><br>
  CONNECT + L7 terminate, secret rewrite, policy.local advisor, middleware pipeline.
</p>
<p align="center">
  <a href="https://github.com/KLORQ/quarshell-proxy/actions/workflows/ci.yml"><img src="https://github.com/KLORQ/quarshell-proxy/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/KLORQ/quarshell-proxy"><img src="https://pkg.go.dev/badge/github.com/KLORQ/quarshell-proxy.svg" alt="Go Reference"></a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0"><img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"></a>
  <a href="https://github.com/KLORQ/quarshell-proxy"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go" alt="Go Version"></a>
</p>
<p align="center">
  <sub>Part of the <a href="https://github.com/KLORQ">cautem / cautem</a> ecosystem</sub>
</p>

---

## Overview

The [policy guide](https://klorq.github.io/sandbox.dev/guides/policy/) covers egress rules; the [security reference](https://klorq.github.io/sandbox.dev/concepts/security/) records current limits.

**quarshell-proxy** is the mandatory egress sidecar for cautem sandboxes. It enforces policy at L4/L7, rewrites credential placeholders, emits OCSF audit lines, and exposes `policy.local` for in-sandbox proposals.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **L4** | CONNECT allowlist from `quarshell-core` engine |
| **L7** | TLS terminate for REST / GraphQL / MCP rules |
| **Secrets** | Placeholder rewrite + gateway secret refresh |
| **Advisor** | `https://policy.local/v1/{policy,denials,proposals}` |
| **Hot reload** | Watch policy file; Apply under RWMutex |

---

## Installation

Use the sibling `go.work` workspace for source builds and run `go test ./...` in this checkout. Published alpha dependencies have not passed a standalone consumer build.

Usually run as the sandbox sidecar (started by `quarshell-driver`), not as a standalone service.

**Requirements:** Go 1.27+

---

## Quick Start

```bash
# From the CLI (host-side debug proxy):
cautem proxy --listen 127.0.0.1:3128 --policy ./policy.yaml
```

On Linux and macOS the listener can also use a private Unix socket, for example
`--listen unix:///run/user/1000/cautem/proxy.sock`. The socket directory must
already exist, belong to the current user, and have no group or other access;
the proxy creates the socket with mode `0600` and removes it on shutdown. VM
workers use this listener with a guest-initiated vsock mapping, keeping the
policy proxy off the guest's network interface.

---

## Package Structure

| Path | Purpose |
|------|---------|
| `proxy/` | Server, MITM, audit, policy.local |
| `proxy/middleware/` | Request pipeline hooks |


---

## Related

| Resource | Link |
|----------|------|
| Roadmap | [ROADMAP.md](./ROADMAP.md) |
| Organization | [https://github.com/KLORQ](https://github.com/KLORQ) |
| Organization overview | [github.com/KLORQ](https://github.com/KLORQ) |
| pkg.go.dev | [`github.com/KLORQ/quarshell-proxy`](https://pkg.go.dev/github.com/KLORQ/quarshell-proxy) |

## License

[Apache-2.0](./LICENSE) © cautem
