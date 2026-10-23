# Standalone module publication and consumer check — 2026-10-23

All 20 Go modules were published in independent `KLORQ/quarshell-*` repositories. New module paths use pre-stable `v0.1.0`; modules with already-published immutable tags kept their existing history and received the next compatible `0.x` version. Modules requiring standalone manifest/checksum fixes received a patch release.

Verification command:

```sh
GOPROXY=direct bash scripts/check-published-deps.sh
```

Result: all 20 modules passed `GOWORK=off CGO_ENABLED=0 go test -mod=readonly ./...`. The direct VCS fallback was required because `proxy.golang.org` temporarily cached `unknown revision` responses for fresh patch tags queried before they were pushed. Go checksum verification remained enabled; the module requirements and `go.sum` files are committed, and the tests make no manifest changes.

Selected tags: `quarshell-core v0.4.2`, `quarshell-proto v0.2.2`, `quarshell-slogx v0.1.5`; all other Go modules are at `v0.1.1` except initial unaffected paths at `v0.1.0`. API remains pre-stable.
