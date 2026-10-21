<h1 align="center">quarshell-providers</h1>

<p align="center">
  <strong>Provider catalog & compose</strong><br>
  Builtin provider profiles and effective-policy composition for sandboxes.
</p>
<p align="center">
  <a href="https://github.com/KLORQ/quarshell-providers/actions/workflows/ci.yml"><img src="https://github.com/KLORQ/quarshell-providers/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/KLORQ/quarshell-providers"><img src="https://pkg.go.dev/badge/github.com/KLORQ/quarshell-providers.svg" alt="Go Reference"></a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0"><img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"></a>
  <a href="https://github.com/KLORQ/quarshell-providers"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go" alt="Go Version"></a>
</p>
<p align="center">
  <sub>Part of the <a href="https://github.com/KLORQ">cautem / cautem</a> ecosystem</sub>
</p>

---

## Overview

For profile configuration and credential handling, see the [provider profile guide](https://klorq.github.io/sandbox.dev/guides/provider-profiles/).

**quarshell-providers** ships YAML provider profiles (Cursor, GitHub, NVIDIA, …) and composes them onto a base policy to produce the effective network/credential set a sandbox runs with.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Catalog** | Builtin profiles under `profiles/` |
| **Compose** | Merge provider endpoints + env keys into base policy |
| **Custom** | Import/override profiles via gateway API |
| **Parity** | OpenShell-style provider attach model |

---

## Installation

For now, build against sibling checkouts through `go.work` and run `go test ./...` here. Published versions need a coordinated dependency update before a standalone consumer build can be recommended.

**Requirements:** Go 1.27+

---

## Quick Start

```go
import (
    "github.com/KLORQ/quarshell-core/policy"
    "github.com/KLORQ/quarshell-providers/provider"
)

base, _ := policy.Load("base.yaml")
prof, _ := provider.LoadFile("profiles/cursor.yaml")
effective, err := provider.EffectivePolicy(base, []provider.Layer{{
    InstanceName: "cursor",
    Profile:      prof,
}}, false)
if err != nil {
    panic(err)
}
_ = effective
```

Profiles: [`profiles/`](./profiles/).

Host discovery uses only `discovery.credentials`, in declaration order, and
collects all non-empty environment aliases. An empty discovery list discovers
no credentials. `source` and `scope` are retained as export metadata; gateway
storage determines their authoritative values.

---

## Package Structure

| Path | Purpose |
|------|---------|
| `provider/` | Load, validate, compose |
| `profiles/` | Builtin YAML catalogs |


---

## Related

| Resource | Link |
|----------|------|
| Roadmap | [ROADMAP.md](./ROADMAP.md) |
| Organization | [https://github.com/KLORQ](https://github.com/KLORQ) |
| Organization overview | [github.com/KLORQ](https://github.com/KLORQ) |
| pkg.go.dev | [`github.com/KLORQ/quarshell-providers`](https://pkg.go.dev/github.com/KLORQ/quarshell-providers) |

## License

[Apache-2.0](./LICENSE) © cautem
