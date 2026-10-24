# Workspace scope and authorization acceptance

- Verified: 2026-10-25 (Europe/Samara)
- Host: Darwin arm64, Go 1.27.3
- Scope: `OS-WORKSPACE` membership/RBAC and workspace-bound resource lookup

## Evidence

- `go test -race ./... && go vet ./...` passed in `quarshell-gateway`.
- gRPC workspace tests verify authenticated subject ownership, anonymous denial, cross-workspace denial, user-visible list filtering, member read rights, member mutation denial and platform-admin access.
- Resource scope regression tests create a sandbox, template and service in one workspace and attempt reads through a different workspace selector. Lookups return not found and a foreign idempotent sandbox delete leaves the original sandbox intact.
- Existing workspace application tests cover durable membership, owner/admin/member/viewer role decisions, deletion and replay after store restart.
- Existing quota service and HTTP API tests cover workspace-keyed quota set/get/check behavior.

## Source digest

SHA-256 over sorted per-file SHA-256 records for the workspace gRPC handlers/tests, workspace service/tests, and sandbox/template/service/quota scope implementation and tests:

`ee71c8a626f25fe16bd42311c972942deaed6707ac604ff39c03cc44fbe11626`
