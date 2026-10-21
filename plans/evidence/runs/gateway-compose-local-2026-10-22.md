# Gateway Compose local acceptance

- Verified: 2026-10-22 (Europe/Samara)
- Host: macOS Docker Desktop, disposable Compose project
- Image: `quarshell-gateway:local`, built locally from the Gateway Dockerfile
- Result: Compose configuration, SQLite startup, HTTP liveness, gRPC listener, and image healthcheck PASS

The isolated deployment used a synthetic token of at least 32 bytes and host
ports `17483` (HTTP) and `17485` (gRPC). The container created
`/var/lib/openshell/state.db`, returned `{"status":"ok"}` from the unauthenticated
`/v1/healthz`, accepted a TCP connection on gRPC, and reached Docker health
status `healthy`. The unique Compose project and its test volume were removed
afterward. The normal quickstart uses host ports 7443 and 7445.
