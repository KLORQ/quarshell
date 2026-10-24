# SDK live Gateway acceptance

- Verified: 2026-10-25 (Europe/Samara)
- Host: Darwin arm64; Go 1.27.3; Node.js 22; Python 3.13
- Scope: Go, TypeScript and Python SDK access to an actual local Gateway process; independent Go consumer build

## Scenario

The acceptance test builds and starts the real `quarshell-gateway` binary with separate ephemeral HTTP and gRPC listeners and a temporary SQLite database. Each SDK performs health/info and workspace create/get/list/delete requests against the same process. Go and Python target the separate gRPC listener; TypeScript uses the generated Connect browser transport over HTTP. The test shuts down the process and removes its temporary state.

## Commands and result

- `QUARSHELL_SDK_GATEWAY_E2E=1 go test -count=1 -v ./tests/integration -run TestPublicSDKAgainstLocalGatewayProcess` in `quarshell-sdk`: PASS. The harness also runs `npm ci`, `npm run build`, `npm run test:gateway`, and `uv run --extra dev pytest -q tests/test_gateway_e2e.py`.
- `GOWORK=off go test -C quarshell-sdk/tests/consumer ./...`: PASS; a consumer outside the workspace builds and tests against the public SDK module.
- `uv run --extra dev pytest -q` in `quarshell-python`: PASS (25 passed, 1 live-Gateway test skipped in the standalone unit run; the live case passed in the integrated harness).

## Source digest

SHA-256 over sorted per-file SHA-256 records for the E2E harness, TypeScript Connect test/package script, Gateway Connect handler and adapter, and Python client plus unit/E2E tests:

`2fc480b8fe6514e99f66a4a682bc4f99376b064b3948b0e8f3c9947eb2f5e09e`

## Limits

This verifies the SDK lifecycle surface and the three client transports. It does not claim all upstream RPCs are implemented; RPC parity is tracked in `OS-API`.
