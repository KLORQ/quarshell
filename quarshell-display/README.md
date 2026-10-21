<h1 align="center">quarshell-display</h1>

<p align="center">
  <strong>GUI / noVNC helpers</strong><br>
  Passworded noVNC surfaces for Debian GUI sandboxes.
</p>
<p align="center">
  <a href="https://github.com/KLORQ/quarshell-display/actions/workflows/ci.yml"><img src="https://github.com/KLORQ/quarshell-display/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/KLORQ/quarshell-display"><img src="https://pkg.go.dev/badge/github.com/KLORQ/quarshell-display.svg" alt="Go Reference"></a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0"><img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"></a>
  <a href="https://github.com/KLORQ/quarshell-display"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go" alt="Go Version"></a>
</p>
<p align="center">
  <sub>Part of the <a href="https://github.com/KLORQ">cautem / cautem</a> ecosystem</sub>
</p>

---

## Overview

The [image reference](https://klorq.github.io/sandbox.dev/reference/images/) describes the GUI image used with this module.

**quarshell-display** helpers wire noVNC publish mode, password generation, and browser open helpers used by `cautem sandbox create --display novnc`.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Mode** | none / novnc parse + defaults |
| **Auth** | Password file helpers for x11vnc / noVNC |
| **UX** | Open published `vnc.html` URL on the host |

---

## Installation

For local development, use the sibling `go.work` workspace and run `go test ./...` here. The existing alpha tag has an older module path; a new tag is needed before standalone `go get` works.

**Requirements:** Go 1.27+. CPU GUI image: `ghcr.io/cautem/cautem/sandboxes/gui`.
An experimental GUI+GPU image is published as
`ghcr.io/cautem/cautem/sandboxes/gui-gpu`; use `cautem sandbox create --gpu
--display novnc` on a Linux NVIDIA host with CDI configured. Driver capabilities
and image libraries alone do not confirm accelerated rendering.

---

## Quick Start

```bash
cautem sandbox create --name gui --from gui --display novnc \
  --workspace . --policy ./policies/default.yaml
# open printed http://127.0.0.1:6080/vnc.html?password=…
```

---

## Package Structure

| Path | Purpose |
|------|---------|
| `novnc/` | Mode parsing, password, open helpers |


---

## Related

| Resource | Link |
|----------|------|
| Roadmap | [ROADMAP.md](./ROADMAP.md) |
| Organization | [https://github.com/KLORQ](https://github.com/KLORQ) |
| Organization overview | [github.com/KLORQ](https://github.com/KLORQ) |
| pkg.go.dev | [`github.com/KLORQ/quarshell-display`](https://pkg.go.dev/github.com/KLORQ/quarshell-display) |

## License

[Apache-2.0](./LICENSE) © cautem
