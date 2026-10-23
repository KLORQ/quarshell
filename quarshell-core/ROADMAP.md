# Roadmap — quarshell-core

Status: pre-stable API · Current module tag: `v0.4.2` · Dependencies are listed in `go.mod`.

Shared policy schema and egress engine.

## This module

| ID | Item | Notes |
|----|------|-------|
| C1 | **MCP / L7 depth** | First-class `protocol: mcp` method/tool matchers |
| C2 | **Policy decision integrations** | Use native OpenShell-shaped policy rules; remote PDP remains an explicit middleware integration |
| C3 | **Policy schema freeze** | Stabilize YAML for alpha consumers; versioned migration notes |
| C4 | **Engine concurrency** | Expand race tests around hot-reload + DecideHTTP |

## Non-goals (alpha)

- Embedding a full OPA SDK as the primary path; policy schema does not claim Rego support


## Release

This module follows its own pre-stable 0.x version sequence. Publish dependencies
first, update consumers to the published tags, and verify with `GOWORK=off`. Never
rewrite a published tag.
