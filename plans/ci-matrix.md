# CI lanes

The repository is a Go workspace with independently versioned modules. The
root CI uses GitHub-hosted runners; this matrix defines current lanes and the
remaining release gates.

| Lane | Runner | Commands | Evidence rows |
| --- | --- | --- | --- |
| Policy/config | GitHub-hosted `ubuntu-24.04` / `macos-14` | `task conformance:check`, `task compat:gateway-config`; pinned upstream differential is a separate verified command | OS-POLICY-SCHEMA, OS-PROVIDERS |
| Go workspace | GitHub-hosted `ubuntu-24.04` / `macos-14` | `task workspace:check`, `task dag:check`, `task test`; standalone strict is a separate verified command | OS-API, OS-SDK |
| Isolation security | GitHub-hosted `ubuntu-24.04` | Linux build, package tests, namespace/cgroup probes; kernel bypass coverage remains open | OS-FS, OS-NET, OS-ISOLATION-PROTO |
| Docker/Podman | GitHub-hosted `ubuntu-24.04` | `task e2e:docker`, `task e2e:podman` (rootful/rootless lifecycle, resource, PTY, userns/binds) | OS-BACKENDS, OS-COMPUTE |
| Kubernetes | GitHub-hosted Linux when a cluster lane is provisioned | kind cluster, RBAC, pod lifecycle, cleanup | OS-BACKENDS, OS-GATEWAY-DEPLOY |
| VM KVM | GitHub-hosted `ubuntu-24.04` probe; live lifecycle runs only with usable non-root `/dev/kvm` | `task vm:live-linux` | OS-BACKENDS, OS-GPU |
| VM HVF | GitHub-hosted `macos-14` probe; live lifecycle runs only when `kern.hv_support=1` | `task vm:live-darwin` | OS-BACKENDS, OS-GPU |
| Windows | GitHub-hosted `windows-latest` | Build the driver workspace; run Windows-compatible driver package tests; report Windows build, `wxc-exec.exe`, and `IsoSessionApp.dll` availability | OS-BACKENDS |
| GPU/VFIO | GPU host | CDI/passthrough allocation and release | OS-GPU |
| Credentials | macOS Docker | Vault/Postgres/SPIRE integration profiles | OS-CRED-API, OS-CRED-DRIVERS, OS-TOKEN |
| Release | macOS + signing hosts | `task release:dry-run`, images, SBOM, signatures, clean install | OS-DISTRIBUTION |
| Evidence promotion | CI | `task evidence:check`, `task evidence:report-check` | all rows |

Local macOS substitutes:

- `task standalone:strict` uses a local module proxy instead of published versions.
- `task credentials:postgres` uses Docker; it is blocked when the Docker VM has no free space.
- `task conformance:upstream` builds the pinned Rust oracle; no upstream binary is shipped.
- `task release:dry-run` produces unsigned binaries, checksums, and an SBOM manifest.

Promotion requires every `required: true` row in
`plans/evidence/matrix.yaml` to be `pass` with a reproducible command and
artifact. `Unsupported`/`Inconclusive` prover results never promote a row.

The hosted platform probes do not count as live backend acceptance when the
runner lacks KVM, HVF, or the MXC runtime. Such runs report the missing host
capability explicitly and leave the corresponding evidence rows unverified.
