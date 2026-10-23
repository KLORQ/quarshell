# Evidence status

Generated from `matrix.yaml` by `tools/cmd/quarshell-tools evidence-report`.

- Baseline: OpenShell v0.1.4
- Upstream SHA: `5a7728c304b5528e954e9ae900cf21f84e1a12b1`
- Rows: 29 (required 29)
- Promotion: NOT READY
- Counts: pass=1, unverified=28

| ID | Owner | Required | Result | Level | Backend | Host | Artifact |
| --- | --- | --- | --- | --- | --- | --- | --- |
| OS-API | gateway + proto | yes | unverified | - | - | - | - |
| OS-COMPUTE | driver + proto | yes | unverified | - | - | - | - |
| OS-CRED-API | credentials | yes | unverified | - | - | - | - |
| OS-MIDDLEWARE | proxy + extensions | yes | unverified | - | - | - | - |
| OS-INTERCEPTORS | gateway + extensions | yes | unverified | - | - | - | - |
| OS-ISOLATION-PROTO | isolation + supervisor | yes | unverified | - | - | - | - |
| OS-GATEWAY-DEPLOY | gateway + CLI | yes | unverified | - | - | - | - |
| OS-SANDBOX | gateway + driver | yes | unverified | - | - | - | - |
| OS-MAIN | supervisor + isolation | yes | unverified | - | - | - | - |
| OS-EXEC | supervisor + SDK | yes | unverified | - | - | - | - |
| OS-WORKSPACE | gateway | yes | unverified | - | - | - | - |
| OS-POLICY-SCHEMA | policy | yes | pass | integration | quarshell-policy + pinned OpenShell v0.1.4 oracle | Darwin arm64; Go 1.27.2; Rust 1.98.0 | plans/evidence/runs/policy-differential-2026-10-22.md |
| OS-POLICY-LIFECYCLE | gateway + supervisor | yes | unverified | - | - | - | - |
| OS-FS | isolation | yes | unverified | - | - | - | - |
| OS-NET | isolation + proxy | yes | unverified | - | - | - | - |
| OS-L7 | proxy + policy | yes | unverified | - | - | - | - |
| OS-CREDENTIALS | credentials + supervisor + proxy | yes | unverified | - | - | - | - |
| OS-TOKEN | credentials | yes | unverified | - | - | - | - |
| OS-PROVIDERS | providers | yes | unverified | - | - | - | - |
| OS-BACKENDS | driver / driver-vm / driver-mxc | yes | unverified | - | - | - | - |
| OS-GPU | compute + vm | yes | unverified | - | - | - | - |
| OS-CRED-DRIVERS | credentials | yes | unverified | - | - | - | - |
| OS-PEER | gateway | yes | unverified | - | - | - | - |
| OS-PROVER | prover | yes | unverified | - | - | - | - |
| OS-OBS | telemetry + владельцы событий | yes | unverified | - | - | - | - |
| OS-CLI | CLI | yes | unverified | - | - | - | - |
| OS-SDK | SDK + client packages | yes | unverified | - | - | - | - |
| OS-DISTRIBUTION | release tooling | yes | unverified | - | - | - | - |
| OS-DOCS | docs + CLI | yes | unverified | - | - | - | - |
