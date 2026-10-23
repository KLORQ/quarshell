<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# ADR 0001: Domain model and compute backend SPI

- **Status:** Accepted and implemented; remaining OpenShell behavior gaps are tracked in the compatibility matrix
- **Date:** 2026-10-16
- **Decision owners:** Quarshell maintainers
- **Scope:** compute backends and Gateway orchestration

## Context

The driver package defines a product-owned request and `Backend` contract,
typed backend identities and capabilities, and optional extension interfaces.
All built-in local implementations register this contract directly; the Gateway
resolves only native Backends. Remote ComputeDriver plugins implement the same
Backend contract over the pinned OpenShell protocol. The VM backend currently
fails closed at Gateway runtime validation because its host-supervisor and
isolation lifecycle is incomplete. The former flat `Spec`/`ComputeDriver`
compatibility API has been removed.

## Decision

1. **The product owns the domain contract.** Orchestration uses
   `BackendCreateRequest`, `Backend`, `BackendInfo`, and typed domain values.
   Transport messages, upstream protobuf types and container-engine request
   types stay outside this contract.
2. **Keep the required lifecycle small.** `Backend` owns identity, capability
   discovery, create validation, create, start, stop, delete, list and inspect.
   Optional behavior is expressed by narrow extension interfaces, such as
   `InteractiveExec`, `FileTransfer`, `ServiceForward`, `LogAccess` and runtime
   extensions. A backend may implement only the behavior it can support.
3. **Capabilities and implementations must agree.** Callers obtain an
   extension through `driver.RequireExtension[T]`. The helper checks both the
   advertised capability and the method implementation and returns a typed
   `UnsupportedError` when either is absent. A caller must not silently
   downgrade a requested security or lifecycle behavior.
4. **Validate before committing state.** Gateway resolves and validates a
   backend create request before writing durable lifecycle state or consuming a
   one-use upload token. Secrets are short-lived operation inputs and are not
   copied into durable create records.
5. **Use direct modular backend contracts.** Every built-in implementation
   registers `Backend` directly and owns its typed create model. Gateway resolves
   native factories only and returns factory errors without fallback. The earlier
   migration-adapter proposal is superseded by the 2026-10-21 direct-port
   decision; the unused flat API cleanup remains tracked under B0.6.
6. **Keep the pinned wire contract.** The generated OpenShell schema and its
   Gateway implementation keep the exact upstream service, field and package
   identifiers. Gateway methods map those requests directly to product domain
   operations. Product-owned API and domain names do not inherit upstream wire
   identifiers. Unsupported operations return typed unsupported results at
   the protocol boundary.
7. **Keep clients independent.** CLI and browser UI are separate API clients.
   The CLI may contain terminal presentation code and call the SDK; browser UI
   uses a typed browser client and Gateway API. Gateway does not import CLI or
   UI packages, and browser UI does not launch the CLI.

## Contract example

This excerpt uses the implemented Go SPI. A backend-specific configuration type
belongs to that backend package; the shared request does not need a generic
serialized driver configuration field:

```go
type Backend interface {
    ID() BackendID
    Capabilities(context.Context) (BackendInfo, error)
    ValidateCreate(context.Context, BackendCreateRequest) error
    Create(context.Context, BackendCreateRequest) (Handle, error)
    Start(context.Context, core.ID) error
    Stop(context.Context, core.ID) error
    Delete(context.Context, core.ID) error
    List(context.Context) ([]Info, error)
    Inspect(context.Context, string) (Info, error)
}

type BackendCreateRequest struct {
    SandboxID core.ID
    Name      string
    Workspace WorkspaceTarget
    Workload  WorkloadSpec
    Resources ResourceLimits
    Mounts    []Mount
    Network   NetworkSpec
    Runtime   RuntimeAssets
    Config    any // typed by the selected backend, not a wire-format map
}

transfer, err := driver.RequireExtension[driver.FileTransfer](
    ctx, backend, driver.CapabilityFileTransfer, "copy into sandbox",
)
if err != nil {
    return err // preserve Unsupported; do not claim the operation succeeded
}
return transfer.CopyTo(ctx, sandboxID, source, destination)
```

The real `BackendCreateRequest` also includes the sandbox name and unsafe-mount
admission flag. Runtime and network values are ephemeral. Implementations call
`ValidateCreate` before allocating backend resources and reject inputs they
cannot safely represent.

## Module and adapter layout

The target ownership boundaries are:

```text
core/                 domain values, policy, validation and typed errors
proto/                product API schemas and generated product clients
driver/                Backend SPI and Docker, Podman and MicroVM adapters
runtime/               guest init, supervisor and guest protocol
proxy/                 egress enforcement and credential mediation
providers/             provider profiles and credential refresh validation
gateway/               auth, orchestration, durable state and API adapters
sdk/                   Go and Python API clients
cli/                   commands, SDK calls and terminal presentation
ui/                    browser application and typed browser API client
```

These are ownership boundaries, not a requirement that every repository be
split immediately. Current repository and Go module paths still use Quarshell
names; their staged rename and release order is documented in the
[rename and release plan](../../reference/renaming-and-release.md).

The Gateway resolver in `quarshell-gateway/internal/httpapi/compute.go` selects
registered native backends. The external OpenShell protocol handler maps once
to the modular create request. A backend owns its typed config and implements
the domain SPI without importing CLI packages or transport schemas.

## Consequences

- A backend can be added behind the SPI without adding backend conditionals to
  API clients or making its configuration part of a shared wire schema.
- Optional features have explicit capability negotiation and can fail closed.
- Native Backend registration is used for Docker, Podman, Kubernetes and VM;
  the former flat `Spec`/`ComputeDriver` compatibility surface is removed.
- VM Gateway provisioning remains fail-closed pending host-supervisor and
  isolation lifecycle parity. Current build evidence does not establish live
  backend acceptance, full feature parity or production readiness.
- At the gRPC compatibility boundary, unsupported behavior maps to the
  protocol's unsupported status. Existing wire identifiers remain stable while
  product-owned APIs can use their own namespace.
- Module, package and binary renames wait for configured target remotes,
  published replacement dependencies and a consumer build without local
  workspace overrides. See the release plan for the cutover and rollback order.

## Verification

The decision is reflected in `driver/backend.go`, the Gateway backend resolver,
the compatibility RPC adapter, the RPC inventory and backend contract tests.
Those tests verify the adapter and the explicit unsupported path. Backend
production acceptance remains governed by the feature matrix and open-gap
register; this ADR does not change their status.
