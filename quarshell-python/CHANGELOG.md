# Changelog

## Unreleased

### Removed

- Remove managed inference route methods and the `InferenceRoute` SDK model; the pinned OpenShell Gateway does not provide inference route APIs.


### Added

- Support named service exposures in Gateway-managed sandbox create requests and return the service URL map.
- Document service authorization modes supported by Gateway service endpoints: strip and bearer passthrough.
- Add public cursor page APIs and all-pages policy revision metadata for sandboxes and OpenShell-backed resource lists; reject repeated cursors and revisions.
- Add platform-admin global policy revision page and all-pages methods to the Python SDK.
- Reject duplicate stable resource identifiers across cursor pages in Python all-pages list methods.
- Support platform-admin all-workspace provider and service pagination in the Python SDK, with deterministic provider ordering by workspace and name.
- Add the generic `CursorPage` and `PolicyRevisionRecord` models; page requests cap their size at 1000 records.
- Add workspace-scoped SSH session creation and token revocation to the Python client; hide session tokens from object representations.
- Add workspace listing, details and member pagination, creation/deletion, and member role management to the Python client.
- Add sandbox service endpoint expose/get/list/delete methods with cursor pagination and authorization mode selection.
- Add inference route get/update/clear with optimistic concurrency and typed global gateway settings access.
- Add filtered sandbox log retrieval, streaming watch with cancellation, and client log append.
- Add sandbox inventory watch with reset snapshot and lifecycle events.
- Add bounded-memory rootfs tar staging upload from a binary stream.
- Add authenticated caller identity lookup through the Control API.
- Add lifecycle operation lookup/listing and paginated audit event retrieval.
- Add managed sandbox registry sync/get/delete and one-time supervisor token rotation.
- Reject repeated sandbox list cursors instead of issuing unbounded page requests.
- Preserve compute driver and runtime status metadata in sandbox summaries.
- Forward sandbox resource, environment, CDI, TTY, template, and driver settings on create; expose durable create operation IDs.
- Expose lifecycle request/operation handles for start, stop, and delete; short lifecycle methods reject unconfirmed completion.
- Add workspace-scoped shared workload template create/get/list/delete to the Python client, including cursor pagination and typed resource fields.
- Add sandbox start/stop methods with workspace and optimistic resource-version checks.
- Add provider CRUD, secret-only credential patches, sandbox attach/detach, provider inventory, and OAuth refresh configuration/rotation methods.
- Add global/sandbox policy reads and optimistic updates, revision history, and policy proposal review methods.
- Add scoped provider profile list/get/create/update/delete with resource-version checks and document size validation.
- Add bidirectional interactive exec with streaming stdin/stdout/stderr, TTY resize frames, and cancellation when the iterator is closed.
- Include partial PEP 561 typing metadata and package the generated protobuf stubs.

### Changed

- Remove duplicate legacy setuptools metadata so builds have a single pyproject.toml source of truth.
- Regenerate Python OpenShell protobuf/gRPC clients from the v0.1.3 pinned source so workspace selectors, cursor pagination, and current execution requests match the Gateway contract.

## [0.1.7] - 2026-10-13

### Changed

- Generate the first-party Python Control API client from the independent `quarshell-proto` module.
- Keep OpenShell generated clients tied to the pinned upstream source revision.

## [0.1.6] - 2026-10-11

### Changed

- Rename the installable package and SDK imports to `quarshell`.

### Changed

- Move overview, sandbox CRUD, and command execution from REST endpoints to native `quarshell.control.v1` and pinned OpenShell gRPC clients. The beta Python API now requires sandbox images for create operations.
- Check in reproducible generated Python protobuf/gRPC clients and declare their runtime dependencies.
- Retain HTTP only for the unauthenticated health probe.

## [v0.1.0-beta.1] - 2026-10-08

### Changed

- Adopt Apache-2.0 licensing and include the project license and notices in package metadata.
- Mark the package as beta; PyPI publication remains pending.
