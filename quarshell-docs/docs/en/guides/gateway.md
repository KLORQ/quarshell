<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Gateway

The gateway is the optional control plane: encrypted credential store, sandbox
registry, observation log rings, and policy proposal workflow.

## Run

```bash
quarshell gateway ensure
quarshell gateway add https://127.0.0.1:17670 --local --name local
quarshell gateway select local
quarshell gateway info
```

Compose: `quarshell-gateway/compose/docker-compose.yml`.

The daemon binary reports its build version with `quarshell-gateway --version`
or `-V`. Release images use their published image version; local builds report
`0.0.0-dev` unless a version is supplied at link time.

### Durable state upgrades

The gateway upgrades its durable state schema on startup. Schema v2 adds stable
service endpoint IDs and preserves each endpoint's domain setting; schema v3
persists service authorization mode and defaults existing records to `strip`.
Older records default to domain-enabled behavior. Back up `state.json`, or both
the PostgreSQL database and `$XDG_STATE_HOME/openshell` (or
`$HOME/.local/state/openshell`), before upgrading. A schema-v2 gateway
rejects a schema-v3 store, so rollback requires restoring the pre-upgrade backup.

Listen address defaults to `127.0.0.1:17670`.
If you set `[openshell.gateway.auth] allow_unauthenticated_users = true` or
pass `--allow-unauthenticated-users`, the Gateway accepts unauthenticated
requests on the configured listener and logs a warning. Use this only in a
trusted development environment.

## Responsibilities

| Surface | Role |
|---------|------|
| Secrets | AES-GCM store; KEK via `QUARSHELL_SECRETS_KEK` or `secrets.kek` |
| Providers | Named instances + profile composition metadata |
| Sandboxes | Registry and lifecycle management through RPC; bounded log ring |
| Sidecar | Resolves secrets over `host.quarshell.internal` |

### Supervisor replicas

For multiple Gateways, set the replica ID to the pod name. In Kubernetes, use
SRV discovery through a headless Service (the Helm chart configures it
automatically):

For multiple pods, configure shared PostgreSQL with `database.existingSecret`
and set `persistence.enabled=false`. SQLite state is local to one replica.

```bash
QUARSHELL_GATEWAY_REPLICA_ID=gateway-0 \
QUARSHELL_GATEWAY_REPLICA_DISCOVERY_SRV='_grpc._tcp.gateway-headless.namespace.svc.cluster.local' \
quarshell-gateway
```

The ring uses 128 virtual nodes per replica and OpenShell-compatible SHA-256
placement. A Gateway sends `SessionRedirect` before registering a local
supervisor owner. During shutdown, it redirects new connections to another
replica. SRV membership refreshes every five seconds; transient DNS failures
preserve the last valid ring. Other environments can provide the same static
`QUARSHELL_GATEWAY_REPLICA_PEERS` list to each replica. With routing unset, the
Gateway keeps single-replica behavior.

## HTTP API reference

The [OpenAPI document](https://github.com/KLORQ/quarshell/blob/main/quarshell-gateway/api/openapi.yaml)
covers health and authentication bootstrap only. Management operations use
the pinned OpenShell RPC contract. SSH byte streams and
supervisor relays use the OpenShell `ForwardTcp`, `ConnectSupervisor`, and
`RelayStream` RPCs. Browser facing sandbox service URLs remain host routed.

Service URLs use `workspace--sandbox--service.<domain>`. The default domain is
`openshell.localhost`; wildcard names in `[openshell.gateway].server_sans` add
custom service domains, with the default retained. Loopback listeners serve
service URLs over HTTP by default even when Gateway TLS is enabled. Set
`enable_loopback_service_http = false` to require HTTPS on loopback.

## Management clients

The CLI, Go and Python SDKs, and browser console use native OpenShell Gateway
RPCs for sandbox lifecycle, inventory, logs, services, workspaces, provider
profiles, policy, settings, identity, SSH sessions, and command execution.
`logs --all` enumerates caller-visible workspaces and opens at most 24 streams
at once. Native gRPC is served on the separate gRPC listener. The generated
TypeScript client uses the OpenShell Connect transport mounted on the Gateway
HTTP listener; both transports reach the same RPC handlers.

The Go Gateway currently authenticates management RPCs with its configured
static bearer token. It does not yet validate OIDC access tokens or map OIDC
role and scope claims to RPC permissions. A browser PKCE login therefore does
not by itself authenticate to this Gateway. Never put the Gateway operator
token or a sandbox supervisor token in browser code.

## When required

| Operation | Gateway |
|-----------|---------|
| Sandbox create, lifecycle, exec, and upload | Required |
| `--provider` attach | Required |
| Egress proxy + rewrite | Required |

## Config

Gateways are recorded in `~/.config/quarshell/config.yaml`:

```yaml
current: local
gateways:
  local:
    url: https://127.0.0.1:17670
```

Local gateways read `auth_token` from the configured Gateway `data_dir`; the
Gateway does not mint tokens through an HTTP login endpoint. The CLI can store
OIDC or edge credentials for client-side flows, but the Go Gateway currently
accepts its configured bearer token for management RPC authentication. Edge
gateways use the `/auth/connect` confirmation flow and carry gRPC over the
`/_ws_tunnel` WebSocket endpoint.

## Related

- [Credentials](./credentials.md)
- [Policy](./policy.md)
- Compose files: `quarshell-gateway/compose/`

## OpenShell gateway TOML

The daemon reads `--config gateway.toml`, then `OPENSHELL_GATEWAY_CONFIG`,
then an optional `$XDG_CONFIG_HOME/openshell/gateway.toml`
(`~/.config/openshell/gateway.toml` fallback). Explicit missing files fail.
Supported values follow flag > environment > file precedence.

Run `quarshell-gateway config preflight [--path gateway.toml]` to parse and
validate the selected configuration without opening the database, creating
state, or starting listeners. To validate daemon flags as well, pass them
after `--`, for example `quarshell-gateway config preflight -- --port 17670`.
This Go preflight checks config loading, selector and auth/TLS relationships,
and Gateway rate-limit arguments. Docker, Podman, and Kubernetes validate
deployment settings through side-effect-free backend validators. VM preflight
also checks that `mke2fs` (or `mkfs.ext4`), `debugfs`, and `e2fsck` are
available and report e2fsprogs 1.43 or newer; these bounded `-V` probes run
only for the explicit preflight command.

Current startup applies the main bind address, installation name in logs,
simple log levels, SSH session TTL, local auth, OIDC/JWKS settings, extension
JWT signing/discovery, TLS certificates, external SNI certificates, mTLS
identity, and builtin or user `provider_profile_sources`. Driver selection supports only
the singular `compute_driver` in TOML, `--compute-driver`, and
`OPENSHELL_COMPUTE_DRIVER`. The deprecated `OPENSHELL_DRIVERS` accepts one
name and emits a migration warning. Flag and canonical environment selection
override it; canonical selection conflicts are rejected. Referenced paths remain literal and
relative to the process working directory. `disable_tls=true` ignores server
certificate material; configuring a client CA at the same time is rejected.
For a remote compute driver on a Unix socket, pair `--compute-driver-socket`
or `OPENSHELL_COMPUTE_DRIVER_SOCKET` with the canonical driver flag or
environment variable; this replaces a compiled backend with that name.

This support is partial. The in-process VM backend consumes `state_dir`,
`default_image`, `bootstrap_image`, `vcpus`, `mem_mib`, and
`overlay_disk_mib`, `rootfs_tar_staging_dir`, `rootfs_tar_max_bytes`, and
`allow_driver_config` (false by default), `krun_log_level` (default 1; values
0–5 map from off to trace), plus `sandbox_uid` and
`sandbox_gid` guest identity overrides. When either override is set, the VM
updates the `sandbox` account in its writable overlay so name-based workload
users resolve to the configured IDs. VM HTTPS Gateway endpoints use
`guest_tls_ca` only; the CA is passed to the host-side guest relay, while
`guest_tls_cert` and `guest_tls_key` are rejected to match OpenShell;
relative `state_dir` paths resolve from the process working directory.
Gateway supplies `default_image`, `state_dir`, `krun_log_level`, vCPU, memory,
and overlay to the VM backend, including their OpenShell defaults. Their
environment variables do not override those Gateway values. Environment
fallbacks apply to bootstrap image, TLS CA, UID/GID, and rootfs tar settings
when the matching deployment field is absent. `OPENSHELL_VM_RUNTIME_DIR` and
`OPENSHELL_VM_IMAGE_PULL_CONCURRENCY` are read directly; layer pull concurrency
uses the upstream default (4) and maximum (16). An empty `default_image` is
valid only when `bootstrap_image` is configured.
Host supervisor binary overrides and enabled proxy, SPIFFE, and GPU features
fail closed until those backend features are ported. Explicit `false` values
for boolean feature flags are accepted as no-ops. GPU memory/vCPU values are
validated but ignored while GPU is disabled.
Unsupported VM deployment fields fail closed. Other
driver-specific runtime settings, supervisor OTLP configuration, and
interceptor-backed provider profile sources remain unported. Provider readiness
receipts, status queries and session-bound supervisor observations are supported;
multi-replica peer relay routing and the Go supervisor reporters, plus complete
readiness/metrics and provisioning-deadline parity, remain open. Configured supervisor middleware and Gateway interceptor endpoints are
supported. Some local and remote compute drivers already work. Supplied unsupported settings fail before the daemon
creates state or opens listeners. The loader also rejects unknown/duplicate
keys, invalid required fields/enums, and a database URL embedded in TOML.
The parser accepts the pinned v0.1.4 schema version 2 and its singular
`compute_driver` selector. The legacy `compute_drivers` list is rejected as an
unknown field. Parsing an upstream fixture does not mean all
of its driver settings are runnable: unsupported backend settings still fail
closed during startup.
Without an OpenShell file, the existing quarshell defaults still apply.
The optional `health_bind_address` and `metrics_bind_address` tables, plus
`OPENSHELL_HEALTH_PORT` / `OPENSHELL_METRICS_PORT` and matching port flags,
start separate listeners. `/healthz` is an empty-body liveness check; `/readyz`
reports cached database readiness. The metrics listener exports
Prometheus request, relay, readiness, and quota metrics.
The gateway-wide gRPC rate limit accepts `grpc_rate_limit_requests` together
with `grpc_rate_limit_window_seconds`; matching `OPENSHELL_GRPC_RATE_LIMIT_*`
environment variables and `--grpc-rate-limit-*` flags are also supported. Both
values must be positive to enable the fixed window; set either to zero to
disable it. Requests over the limit receive `RESOURCE_EXHAUSTED`.
An optional `[openshell.gateway.otlp]` table enables gateway HTTP request traces
over OTLP/gRPC. Set `endpoint` to an OTLP collector URL such as
`https://otel-collector.example:4317`; `service_name` defaults to
`quarshell-gateway`. Spans contain the HTTP method, response status, request
ID, and RPC service/method/status when applicable. Request paths, query strings,
other headers, and bodies are omitted.
An invalid endpoint disables trace export and is reported in the Gateway log;
the Gateway continues serving requests.

The TOML loader recognizes the OpenShell OIDC issuer, audience, JWKS, and role
claim fields, but the Go Gateway does not currently wire them into its RPC
authentication path. OIDC signature validation exists in the runtime identity
package; it is not active in the Gateway. Configure `QUARSHELL_TOKEN` for the
current management API, and do not treat OIDC settings as an enabled Gateway
authentication mechanism.

TLS can load a server certificate/key pair. When `--tls-client-ca` is set, the
HTTP listener requires a client certificate issued by that CA. The certificate
is not mapped to a Gateway user or operator role, so management RPCs still
require the configured bearer token. The gRPC listener uses the server TLS
certificate but does not request client certificates. The parsed OpenShell
`mtls_auth` operator-role settings are not wired into the Gateway runtime yet.
Automatic local PKI generation and watched certificate reload are also
pending.

The edge WebSocket tunnel is disabled by default. Enable it with
`enable_websocket_tunnel = true`, `OPENSHELL_ENABLE_WEBSOCKET_TUNNEL=true`, or
`--enable-websocket-tunnel true`. The tunnel forwards to a loopback-only HTTP/2
listener.

By default, profiles come from both the built-in and stored user catalogs. The
source list can restrict that selection, for example:

```toml
[openshell.gateway]
provider_profile_sources = [{ type = "builtin" }]
```

An interceptor profile source is rejected until the gateway interceptor
runtime and its catalog protocol are implemented.
