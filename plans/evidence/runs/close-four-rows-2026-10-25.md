# Four-row closure evidence

- Verified: 2026-10-25
- Host: Darwin 27.0.0 arm64, Go 1.27.3
- Branch: `main`
- Scope: OS-PROVER, OS-PROVIDERS, OS-CRED-DRIVERS, OS-SANDBOX

## OS-PROVER

- Backend: Z3 5.3.1
- Command: `go test -count=1 -race ./prover`
- Result: PASS, including native SAT/UNSAT containment integration and synthetic timeout/missing-solver failures.
- Source digest: `97272f9eaed5fe9bdb249987bbf7a30f517c7204f44083392972d0f96a13f311` (SHA-256 of sorted per-file SHA-256 records for `quarshell-policy/prover/*.go`).
- Finding fixed: the SMT counterexample query checked `allowed \ requested`; containment requires checking `requested \ allowed`. Native Z3 integration exposed the reversed relation.
- Limits: broader OpenShell policy corpus remains covered by the separate conformance evidence; unsupported network features fail closed.

## OS-PROVIDERS

- Backend: pinned OpenShell v0.1.4 examples and bundled provider catalog.
- Command: `go test -count=1 -race ./provider`
- Result: PASS. All 17 upstream YAML examples are checksum-verified against the committed fixture manifest and parsed with preserved IDs and discovery credentials. Built-in catalog and existing profile parse/composition/discovery tests pass.
- Source digest: `1296598f165b73a1ac3a6f3397bc6d25e4d182e4abff5204172a2fe9ef33ff5e` (sorted per-file SHA-256 records for provider Go files, pinned upstream fixtures, and bundled profile YAML files).
- Limits: workspace-scoped profile list/import semantics are tracked in OS-WORKSPACE and OS-API.

## OS-CRED-DRIVERS

- Backend: PostgreSQL 16 in Docker; Kubernetes fake client; Vault-compatible `httptest` server.
- Command: `QUARSHELL_POSTGRES_URL=postgres://postgres:<password>@127.0.0.1:<mapped-port>/postgres?sslmode=disable go test -count=1 -race ./credentialstore`
- Result: PASS. PostgreSQL integration covered create/read, conflict, encrypted material, rotation, wrong-key rejection, deletion and outage classification. Kubernetes and Vault test doubles cover adapter lifecycle, auth, rotation/conflicts and outage mapping.
- Source digest: `f52f0c146de2d199b26c37bc0a7059c7c31fb7d28eb5d2bf164d802165e256d3` (sorted per-file SHA-256 records for `quarshell-gateway/credentialstore/*.go`).
- Fix: Vault transport failures now map to `Unavailable` without exposing request details.
- Limits: no live Kubernetes cluster or Vault service was part of this run.

## OS-SANDBOX

- Backend: durable SQLite store and deterministic fake runtime driver.
- Command: `go test -count=1 -race ./internal/application/sandbox ./internal/application/reconcile`
- Result: PASS. SQLite restart replay preserves one sandbox and one outbox event; reconciliation recovers a persisted `requested` sandbox, handles a lost create response without duplication, and retries after a runtime outage.
- Source digest: `224896603f92d42ed671d8a994d87715a25b668baf14dcbcd694d226e30a4391` (sorted per-file SHA-256 records for sandbox, reconcile, lifecycle, Docker/Kubernetes inspect, and VM state Go files).
- Safety fix: reconciliation creates only after a typed not-found inspection result. The bundled Docker, Kubernetes, and VM drivers classify absent sandboxes; other inspect errors stop reconciliation before creation.
- Limits: live runtime backend acceptance remains tracked in OS-BACKENDS.

## Batch checks

- `go test -race ./... && go vet ./...` passed in `quarshell-gateway`.
- `go test -count=1 -race ./... && go vet ./...` passed in `quarshell-policy` after the prover fix.
