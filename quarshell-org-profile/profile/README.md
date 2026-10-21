<h1 align="center">cautem</h1>

<p align="center">
  <img src="./cautem-org-avatar.png" alt="cautem logo" width="144">
</p>

<p align="center">
  <strong>Agent sandboxes in Go</strong><br>
  Policy-bound containers for Cursor, Claude, Codex — and anything you bring.
</p>

<p align="center">
  <a href="https://klorq.github.io/sandbox.dev/"><img src="https://img.shields.io/badge/docs-klorq.github.io-3F51B5?logo=materialformkdocs&logoColor=white" alt="Documentation"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"></a>
  <a href="https://github.com/KLORQ/quarshell-cli"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://github.com/KLORQ/quarshell-cli/releases"><img src="https://img.shields.io/badge/status-beta-orange" alt="beta"></a>
</p>

---

## Documentation

**[klorq.github.io](https://klorq.github.io/sandbox.dev/)** — shared docs for all modules (EN / RU).

| | |
|--|--|
| English | [Home](https://klorq.github.io/sandbox.dev/) · [Get started](https://klorq.github.io/sandbox.dev/get-started/) |
| Русский | [Главная](https://klorq.github.io/sandbox.dev/ru/) · [Быстрый старт](https://klorq.github.io/sandbox.dev/ru/get-started/) |
| Docker | [EN](https://klorq.github.io/sandbox.dev/providers/docker/) · [RU](https://klorq.github.io/sandbox.dev/ru/providers/docker/) |
| Podman | [EN](https://klorq.github.io/sandbox.dev/providers/podman/) · [RU](https://klorq.github.io/sandbox.dev/ru/providers/podman/) |

Source: [klorq.github.io](https://github.com/KLORQ/sandbox.dev) (Material for MkDocs).

---

## cautem

**cautem** runs coding agents inside hardened Docker sandboxes. YAML policy controls filesystem and egress; an egress sidecar enforces it; an optional gateway provides registry, provider compose, and relayed exec.

```bash
curl -fsSL https://raw.githubusercontent.com/cautem/quarshell-cli/main/install.sh | bash
cautem sandbox create --name demo --workspace . --policy policies/default.yaml
```

### Repositories

| Repo | Role |
|------|------|
| **[klorq.github.io](https://github.com/KLORQ/sandbox.dev)** | Shared docs → [site](https://klorq.github.io/sandbox.dev/) |
| [quarshell-cli](https://github.com/KLORQ/quarshell-cli) | `cautem` CLI, policies, agent images |
| [quarshell-core](https://github.com/KLORQ/quarshell-core) | Policy schema + egress engine |
| [quarshell-providers](https://github.com/KLORQ/quarshell-providers) | Provider profiles + compose |
| [quarshell-proxy](https://github.com/KLORQ/quarshell-proxy) | Egress sidecar + `policy.local` |
| [quarshell-driver](https://github.com/KLORQ/quarshell-driver) | Docker / Podman compute drivers |
| [quarshell-runtime](https://github.com/KLORQ/quarshell-runtime) | Sandbox glue, `cautem-init`, images |
| [quarshell-gateway](https://github.com/KLORQ/quarshell-gateway) | RPC control-plane daemon |
| [quarshell-display](https://github.com/KLORQ/quarshell-display) | noVNC helpers |
| [quarshell-sdk](https://github.com/KLORQ/quarshell-sdk) | Go client |
| [quarshell-python](https://github.com/KLORQ/quarshell-python) | Python client |
| [slogx](https://github.com/KLORQ/quarshell-slogx) | Structured slog helpers |

### Images (GHCR)

```text
ghcr.io/cautem/cautem/gateway
ghcr.io/cautem/cautem/sandboxes/{base,gui,gpu,cursor,claude,codex}
```

### Design

- **Multi-repo** — each module is its own git remote (not a monorepo)
- **Shared docs** — one Material for MkDocs site for all modules
- **Default deny** — L4/L7 egress allowlists in `quarshell-core`
- **OpenShell-shaped** — host-gateway alias `host.cautem.internal`, provider attach, BYOC

Apache-2.0 © cautem
