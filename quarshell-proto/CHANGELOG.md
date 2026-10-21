# Changelog

## [Unreleased]

### Changed

- Add service exposure options and returned service URLs to the Control sandbox create contract.
- Include the effective service authorization mode in Control catalog service summaries for the browser UI.
- Regenerate pinned OpenShell protobuf clients in CI and reject drift in materialized schemas and all generated Go/TypeScript sources.
- Generate Control and OpenShell TypeScript bindings from one template with Node ESM import extensions; clean stale generated sources and build output before packaging.
- Include the Apache-2.0 license and workspace NOTICE in the TypeScript npm package.

## [v0.2.0] - 2026-10-16

### Added

- Add workspace quota contract messages and RPCs for reporting and updating workspace limits.
- Add `AttachMainProcess` bidirectional streaming RPC for canonical workload stdin, output, exit status, and TTY resize.
- Add portable CPU, memory, GPU, environment, driver configuration, TTY, PID limit, and CDI device fields to sandbox creation requests.
