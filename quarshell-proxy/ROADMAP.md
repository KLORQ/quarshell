# Roadmap — quarshell-proxy

Status: pre-stable API · Current module tag: `v0.1.1` · Dependencies are listed in `go.mod`.

## This module

| ID | Item | Notes |
|----|------|-------|
| X1 | **JWT verify** | Cryptographic JWKS/HMAC before credential rewrite |
| X2 | **Secret backends** | Bridge Vault/cloud SM into sidecar `SecretStore` |
| X3 | **policy.local ↔ gateway** | Reliable proposal sync when gateway is present |
| X4 | **MCP terminate** | Policy-aware MCP frames |


## Release

This module follows its own pre-stable 0.x version sequence. Publish dependencies
first, update consumers to the published tags, and verify with `GOWORK=off`. Never
rewrite a published tag.
