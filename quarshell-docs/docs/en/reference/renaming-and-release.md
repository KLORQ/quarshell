<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Quarshell product and release contract

Quarshell is the product name. The GitHub owner remains `quarshell`; the supported
product repositories and Go modules use the `quarshell-*` names. OpenShell is an
upstream compatibility target, not a Quarshell product alias.

## Compatibility policy

Quarshell makes no backward-compatibility promise for earlier Quarshell or
Quarshell releases during this pre-beta cutover. The beta will have one current
CLI, API/schema, package set, configuration layout, environment-variable
namespace and runtime resource identity. Old module paths, binary aliases,
configuration names, API routes, database formats, labels and resource
adoption are not supported. We will not ship dual readers, fallback paths,
compatibility adapters or a Quarshell-to-Quarshell migration tool. Existing
installations require a clean install; users must back up any data they need
outside Quarshell before replacing an older build. The new release must not
silently delete unrelated old host resources.

Breaking changes are allowed until the first production beta. After that beta,
only the explicitly documented Quarshell beta contract is supported; no
compatibility is implied for earlier pre-beta builds.

## OpenShell compatibility target

The compatibility target is the latest stable OpenShell release, pinned by tag
and full commit SHA. As of 2026-10-19, the upstream Releases page marks v0.1.4
as the latest stable release; the moving `main`/`dev` build is not the target.
The active baseline is [OpenShell v0.1.4](https://github.com/NVIDIA/OpenShell/releases/tag/v0.1.4),
commit `5a7728c304b5528e954e9ae900cf21f84e1a12b1`.

“Full compatibility” means behavioral parity for every supported OpenShell
surface, including public RPCs and stream semantics, CLI commands/options,
configuration/defaults/errors, provider and credential lifecycle, policy and
filesystem/network/process boundaries, Docker/Podman/MicroVM behavior, SDK
semantics, TUI flows, and installable artifacts. Matching protobuf descriptors
or command names alone is insufficient. Every upstream capability must have a
Quarshell implementation and a passing black-box/conformance test. A missing or
partially implemented required capability blocks the beta; it must not be
hidden behind a compatibility adapter or reported as supported.

When OpenShell publishes a newer stable release, its pin and full inventory must
be updated and qualified as a deliberate release change. Do not track an
unpinned branch or silently combine contracts from multiple upstream versions.

## Canonical names and cutover

- Go imports and module dependencies use only `github.com/KLORQ/quarshell-*`.
- Product commands, package metadata, config paths, environment variables,
  labels, images and newly created runtime resources use Quarshell names only.
- OpenShell wire names remain unchanged where exact upstream protocol
  compatibility requires them.
- Remove old Quarshell aliases, old-path imports, dual-name environment handling,
  legacy config/database readers and old-resource adoption from the supported
  product path. Do not rename upstream-owned protocol identifiers.
- The release candidate is built from one immutable 15-repository source lock.
  No local `replace`, sibling checkout, dirty worktree or baseline-only lock is
  allowed in the published consumer graph.

## Release status

The current source lock is still `baseline-only`, and the compatibility matrix
contains open behavior and host-security gates. This contract does not claim
that full parity has been achieved. Beta publication remains blocked until the
full conformance inventory, backend/security acceptance, clean published
consumer build, signed artifacts and installable-release tests all pass.
