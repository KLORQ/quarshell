# GitHub-hosted Linux acceptance

- Verified: 2026-10-22 (Europe/Samara)
- Runner: GitHub-hosted `ubuntu-24.04`; no self-hosted runner used
- Workflow run: https://github.com/KLORQ/quarshell/actions/runs/39679621940
- Result: all jobs PASS
- Build/test: workspace and DAG checks, Linux isolation build/tests, full `task test`, policy conformance
- Docker: lifecycle acceptance PASS
- Podman: rootful and rootless E2E PASS, including userns/bind ownership, proxy/network probes, and rootless recovery
- Expected capability skips: rootful `no-map` (Podman supports it rootless only); rootless explicit `private` ID maps

## Final snapshot and backend events

- Linux final snapshot: https://github.com/KLORQ/quarshell/actions/runs/39684405826 — build/test, Docker E2E, and Podman E2E all PASS on GitHub-hosted `ubuntu-24.04`.
- Current clean snapshot (`3b42b27064d8e64e3240b62f8be146dcb274a11a`): https://github.com/KLORQ/quarshell/actions/runs/39687281332 — build/test, Docker E2E, and rootful/rootless Podman E2E all PASS.
- Latest Linux recheck: https://github.com/KLORQ/quarshell/actions/runs/39689935058 — build/test, Docker E2E, and rootful/rootless Podman E2E all PASS on GitHub-hosted `ubuntu-24.04`.
- Latest macOS recheck: https://github.com/KLORQ/quarshell/actions/runs/39689934983 — build/test PASS on GitHub-hosted `macos-14`; signing correctly skipped on a branch run.
- Security-hardened workflow recheck on snapshot `a5b1d013b03b259ec0da03712c9ee15bfb629309`: Linux build/test + Docker/Podman E2E PASS (https://github.com/KLORQ/quarshell/actions/runs/39692425422); macOS build/test PASS and signing skipped (https://github.com/KLORQ/quarshell/actions/runs/39692425519). Checkout credentials are not persisted; only the tag-only publish job receives `contents: write`.
- macOS final snapshot: https://github.com/KLORQ/quarshell/actions/runs/39684411702 — build/test PASS on GitHub-hosted `macos-14`.
- After pinning all workflow actions to immutable full commit SHAs, Linux recheck: https://github.com/KLORQ/quarshell/actions/runs/39685480322 — build/test, Docker E2E, and Podman E2E all PASS.
- Pinned-action macOS recheck: https://github.com/KLORQ/quarshell/actions/runs/39685487364 — build/test PASS on GitHub-hosted `macos-14`; signing correctly skipped on a branch run.
- Final snapshot macOS check (`3b42b27064d8e64e3240b62f8be146dcb274a11a`): https://github.com/KLORQ/quarshell/actions/runs/39687281804 — build/test PASS; signing correctly skipped on a branch run.
- Release workflow cross-build dispatch: https://github.com/KLORQ/quarshell/actions/runs/39685548044 — all 10 cross-builds PASS; upload-artifact initially found no files because its hidden-file filter excluded `.release-dry-run/`.
- Fixed release artifact run: https://github.com/KLORQ/quarshell/actions/runs/39688286882 — 13 files uploaded as `quarshell-release` (artifact ID `12387771219`, 258,620,409 bytes; upload digest `4c69bc481725143656e14a7b21dab6c0ab299a79f8128a935c0593f7419add45`). Downloaded archive verified: 10 manifest binaries present, all SHA256 checks pass, SBOM and manifest included. Binaries are unsigned; tag-only `publish` was skipped.
- Release verifier is now part of CI. https://github.com/KLORQ/quarshell/actions/runs/39690843855 — hosted job verified all 10 expected binary names, manifest SHA256/size entries, `SHA256SUMS`, and 10 SBOM records before uploading all 13 files. Artifact ID `12388423459`, 258,620,429 bytes, upload digest `44173cefacdae97fb5c30997d6843284f5eeab4904f782ff0f2af7c489157562`. Tag-only `publish` was skipped.
- Security-hardened release workflow recheck: https://github.com/KLORQ/quarshell/actions/runs/39692425723 — cross-build, in-workflow verifier, and artifact upload PASS; artifact ID `12390280482`, 258,620,417 bytes, upload digest `fef4ce50941cd2204f85086e40d12e37e2b816bde7657b634a6657811843d39e`. Tag-only `publish` was skipped.
- Secret hygiene recheck on snapshot `57f92ebd91c76763bee93c9e9f3189f9af1edaf5`: pinned Gitleaks CLI scanned all reachable Git history and found no leaks ([run](https://github.com/KLORQ/quarshell/actions/runs/39694038181)); macOS build/test PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39694038224)); Linux build/test, Docker E2E, and rootful/rootless Podman E2E PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39694038417)).
- Latest acceptance on snapshot `b80f8f69809b1358e9c34e0909924a957e85f6d9`: Linux build/test, Docker E2E, rootful/rootless Podman E2E, and Gateway image + clean Compose install PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39697200751)); macOS build/test PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39697208638)); full-history Gitleaks PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39697154907)).
- Latest acceptance on snapshot `a2cf09b1be01495c1559e4e9a548b5a397eba47f`: Linux build/test, Docker E2E, rootful/rootless Podman E2E, Gateway image + clean Compose install, strict docs PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39698862105)); macOS build/test PASS, signing skipped because signing secrets are not configured ([run](https://github.com/KLORQ/quarshell/actions/runs/39699156427)); full-history Gitleaks PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39698844141)).
- Latest code acceptance on snapshot `7ba6b46ae2a259163edeac8ca0bf9dad3624746a`: Linux build/test, Docker E2E, rootful/rootless Podman E2E, Gateway image + clean Compose install, strict docs PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39705921163)); macOS build/test PASS, signing skipped because signing secrets are not configured ([run](https://github.com/KLORQ/quarshell/actions/runs/39705926419)); full-history Gitleaks PASS ([run](https://github.com/KLORQ/quarshell/actions/runs/39705899255)).
- Docker Engine delivers the live `exec_create` container event; Podman 6.1.4 delivers its native `exec` container status. Both E2E checks verify sandbox identity and event type; raw actor attributes are omitted by the adapter.

## Egress evidence (Docker/Podman scope)

The hosted E2E job also exercises the data path, beyond lifecycle success:

- Docker `TestDockerEngineE2E`: allowed authenticated upstream-proxy CONNECT;
  denied policy TCP/UDP probes; direct IPv4 and IPv6 host-gateway probes fail
  closed while the sandbox is attached to its isolated dual-stack network.
- Podman `TestPodmanEngineE2E`: allowed policy-proxy CONNECT; denied policy
  TCP/UDP probes; direct host-gateway access is denied both with the proxy
  running and after it is killed. This ran against rootful and rootless Podman.
- The runner log records `PROXY 200 Connection Established`, `PROXY 403
  Forbidden`, and `BLOCKED` results for denied probes in the successful job.

This is evidence for Docker/Podman network boundaries and proxy policy on the
tested GitHub-hosted Linux profile. The final snapshot also verifies delivery
of one live exec event from each backend. It does not prove host-level
raw-socket mediation, DNS/UDS bypass resistance, or full backend event parity.

This evidence covers the hosted Linux Docker/Podman lane. It does not promote
the complete `OS-BACKENDS` row, which also includes Kubernetes, VM, Windows
MXC, and GPU profiles.
