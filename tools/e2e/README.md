# Engine end-to-end tests

This directory contains the opt-in E2E suite for real Docker-compatible engines.
The `docker.sh` lane starts a disposable Docker-in-Docker daemon in a privileged
container. The daemon API stays on its private Unix socket inside that container;
the host only needs a working Docker CLI and daemon to create/remove the outer
test container. Its image store and containers disappear when the lane exits.

The gateway E2E test runs against native Docker or Podman's Docker-compatible
API. The Podman lane runs rootful lifecycle tests and a rootless user-namespace
matrix in separate Podman services inside the disposable outer container. It
checks `private`, `auto`, `host`, `no-map`, and `keep-id` by inspecting
`/proc/self/uid_map`, `/proc/self/gid_map`, and effective IDs. The rootful lane
covers explicit maps; strict mode permits only rootful `no-map` and rootless
`private` as documented capability skips. A sandbox image and a root-owned proxy image are built once, transferred into
the isolated engine, and removed from the host Docker image store at cleanup. The
test creates a sandbox through the gateway RPC handler, waits for the
supervisor relay, creates and revokes an SSH session through the pinned
OpenShell gRPC API, opens the public SSH relay, authenticates, and executes
`id -un`. Each lane also checks its
driver lifecycle directly: create, start, inspect, exec, stop, and delete. The
Docker/Podman lifecycle corpus verifies immutable image ID, CPU/memory/PID
limits, bind mount read/write behavior, named-volume persistence across
stop/start, tmpfs mode, runtime UID, no-new-privileges, dropped capabilities,
container logs, and resource cleanup. Rootless Podman uses a separate relay
endpoint via `host.containers.internal` because its sandbox network namespace
cannot use the outer container's gateway address directly. The nested rootless
lane does not expose a separate PID namespace to the sandbox, so its PID 1
assertion is reported as a capability limitation; Docker and rootful Podman
still require PID 1 to be the Go supervisor.

## Canonical layout

- `docker.sh`: isolated Docker-in-Docker test lane.
- `docker-desktop.sh`: direct Docker Desktop workspace bind and persistence acceptance on macOS.
- `podman.sh`: Podman service in a disposable outer container.
- `lib.sh`: shared cross-build and sandbox fixture helpers.
- `../..` repository modules: opt-in gateway and driver Go E2E tests.

## Run

From the workspace root:

```sh
task e2e:docker
task e2e:docker-desktop # macOS Docker Desktop host-path acceptance
task e2e:podman
task e2e # both lanes
```

The Docker Desktop lane runs a Linux test runner against the Desktop daemon via
the Docker socket. Its temporary workspace lives under `.e2e-tmp` so the
runner and daemon use the same macOS host path. It checks host-to-sandbox reads,
sandbox-to-host writes, and persistence across stop/start. It complements the
isolated Docker Engine lifecycle and policy suite; it does not cover the
daemon-side IPv6 bridge manipulation or the host bind-path race.

The scripts require Go, `curl`, and a running Docker daemon. The outer daemon
must allow privileged containers because Docker-in-Docker needs nested
namespaces and mounts.

The Docker and Podman lanes run as separate required jobs in the provider
compatibility CI workflow. The `required` check fails if either backend lane
or the workspace unit/contract suite fails. Run `task compat:release` locally
to execute the common checks, provider fixture checks, and both disposable
engine lanes. Each harness also requires the expected `--- PASS` marker from
every mandatory test binary, so a successful process with an unexpected Go
test skip cannot silently satisfy the compatibility gate.
Remaining compatibility scenarios and known runtime limits are tracked in
[OpenShell compatibility](https://klorq.github.io/sandbox.dev/reference/openshell-compatibility/).
The main Podman lane pins Podman 6.1.4 and requires its Gateway/proxy scenarios
to pass. A separate legacy-version run may record an expected fail-closed
capability error, but it does not satisfy this required lane.

The Docker lane can use a different daemon image with `QUARSHELL_DOCKER_E2E_IMAGE`.
The Podman lane builds the same pinned Podman 6.1.4 Fedora image used by its
CI workflows and requires every Gateway/proxy scenario to pass. Use
`QUARSHELL_PODMAN_IMAGE` only to point at a verified image that reports exactly
version 6.1.4. Set `E2E_KEEP_TMP=1` to keep
cross-compiled helpers and the test binary for debugging. Both lanes remove their
outer container and temporary artifacts on exit.

The combined Podman harness assigns separate subordinate ID ranges to the
rootful account (`100000:65536`), rootless `podman` account
(`200000:65536`), and `containers` account (`300000:65536`). It rejects
overlapping or out-of-range values before starting Docker. The standalone
rootless CI service uses `200000:65536`; these are test fixture ranges, not a
claim that the host's production `/etc/subuid` or `/etc/subgid` is configured.

Each nested engine container has a shared default budget of 6 GiB RAM, 4 CPUs,
and 1024 PIDs. Override these with `QUARSHELL_E2E_CONTAINER_MEMORY`,
`QUARSHELL_E2E_CONTAINER_CPUS`, and `QUARSHELL_E2E_CONTAINER_PIDS` when qualifying a
different host size. The tmpfs sizes are upper bounds inside this container;
they do not reserve RAM in advance and cannot exceed the container memory cap.

The gateway test runs as a Linux binary inside the engine container so nested
sandbox containers can reach the gateway listener over their container network.
It is deliberately skipped by ordinary `go test ./...` runs unless
`QUARSHELL_ENGINE_E2E=1` is set by one of these harnesses.
