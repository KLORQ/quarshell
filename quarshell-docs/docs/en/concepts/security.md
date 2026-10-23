<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Security

| Boundary | Trust |
|----------|-------|
| Host Engine / Desktop | Trusted operator machine |
| Sandbox container | Untrusted agent |
| Egress proxy | Trusted sidecar |
| Gateway secrets store | Trusted control plane |
| noVNC / display | Operator loopback only |

## Defaults

- No `docker.sock` inside the sandbox
- Workspace bind with deny-list for `$HOME`, `~/.ssh`, `~/.aws`, `~/.cursor`
- Default-deny network; allowlist from policy + composed providers
- Secrets as `quarshell:resolve:env:…` rewritten on egress only
- Docker default seccomp kept; `no-new-privileges`; `CapDrop=NET_RAW`
- Landlock path rules via `quarshell-init` when kernel ABI ≥ 1 (profile follows the available ABI up to V5, or V7 on ABI ≥ 7; often ABI 0 on Desktop)
- Denied-access kernel audit when Landlock ABI ≥ 7; events go to the Linux kernel audit subsystem and are not automatically copied into `quarshell logs`
- With the OpenShell gRPC supervisor relay on Linux, the supervisor sends OCSF process launch/exit observations for the main workload and `/proc`-observed descendants to gateway logs. When `[openshell.gateway.ocsf_log]` is configured, the Gateway also writes these observations as OCSF Process Activity JSONL. `SIGCHLD` and `wait4` detect exits of the main process and supervisor-adopted children promptly; the 250 ms `/proc` scan remains a fallback. The relay batches events for up to 25 ms. Network time and retries still prevent a guaranteed 250 ms gateway-delivery bound. Short-lived children reaped by their own parent and `exec` replacements can still be missed. Only executable basenames are recorded, without argv or environment values. This is not a complete process or filesystem audit stream.

The egress proxy resolves a destination once, applies SSRF and `allowed_ips`
checks, and connects to a filtered IP. When an upstream corporate HTTP proxy is
configured, the proxy receives that IP as the CONNECT target, so it does not
resolve the hostname a second time. Reserved, loopback and link-local addresses
stay blocked even when a policy specifies `allowed_ips`. The upstream proxy
remains part of the trusted network boundary. Setting
`QUARSHELL_PROXY_CONNECT_BY_HOSTNAME=true` is rejected when an upstream proxy is
used because it would bypass the filtered IP.

On Docker Desktop for macOS, the proxy may not receive a reliable peer binary
identity. A binary-specific policy needs a platform that can supply that
identity; container boundaries and mount restrictions still apply.

Caller supplied backend configuration is disabled by default. When an operator
enables it, named volumes are admitted only when engine labels bind them to the
verified workspace; Quarshell records and rechecks their identity before
restart. Host bind and image mounts remain unavailable while resource admission
is enabled because their sources do not have a trusted label resolver. See the
[Docker](../providers/docker/prerequisites.md#caller-supplied-driver-configuration)
and [Podman](../providers/podman/prerequisites.md#caller-supplied-driver-configuration)
configuration examples.

## Boundaries still being verified

The October 2026 security review fixed known paths through the Docker bridge gateway, live CONNECT tunnels after policy updates, missing SSH hardening wrappers, Docker inspect errors, reserved mount targets, and gateway-owned credentials reappearing from host environment. Those fixes have focused tests. The following runtime checks remain open:

- Confirm that `UpdateConfig` changes the decision of a running proxy on Docker and on rootful and rootless Podman, including gateway or proxy restart.
- Exercise the native host-gateway blocking route on Podman 6.x. Proxy-backed sandbox creation is rejected on Podman versions that cannot enforce that route.
- Test the Docker IPv6 path on a dual-stack Engine and harden workspace bind handling against a path-changing race between validation and mount.

These limits matter when a policy relies on immediate access revocation. See [OpenShell compatibility](../reference/openshell-compatibility.md) for the wider contract status.

## Credentials

Prefer `provider create` over `--env`. Cursor uses `agent login` instead of
injecting `CURSOR_API_KEY`. See [Credentials](../guides/credentials.md).

SPIFFE token-grant access tokens are cached only in proxy memory. The cache
holds at most 256 entries and rejects tokens larger than 64 KiB. Its cache key
includes the full non-secret grant configuration and request target, so a
configuration change does not reuse the previous grant. Rotating a secret under
the same credential key does not immediately purge tokens already issued; their
provider expiry and revocation semantics still apply. Cache lifetime never
exceeds the provider token lifetime. When provider expiry sets that upper bound,
the cache subtracts 30 seconds where possible; an explicitly shorter cache TTL
is preserved.

## Resource hygiene

Uncapped guests and unbounded container logs inflate Docker Desktop VMs. Use
`--memory`, soft defaults, and default json-file rotation:

- [Docker resources](../providers/docker/resources.md)
- [Docker logging](../providers/docker/logging.md)
