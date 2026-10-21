#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIN_FILE="$ROOT/tools/upstream/UPSTREAM_SHA"
VERSION_FILE="$ROOT/tools/upstream/UPSTREAM_VERSION"
if [[ ! -f "$PIN_FILE" ]]; then
	echo "missing pinned OpenShell SHA: $PIN_FILE" >&2
	exit 1
fi
if [[ ! -f "$VERSION_FILE" ]]; then
	echo "missing pinned OpenShell version: $VERSION_FILE" >&2
	exit 1
fi
PIN="$(tr -d '[:space:]' < "$PIN_FILE")"
PINNED_VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
if [[ ! "$PIN" =~ ^[0-9a-f]{40}$ ]]; then
	echo "invalid pinned OpenShell SHA in $PIN_FILE: $PIN" >&2
	exit 1
fi
if [[ ! "$PINNED_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "invalid pinned OpenShell version in $VERSION_FILE: $PINNED_VERSION" >&2
	exit 1
fi
EXPECTED="${PIN:0:12}"
CHECKOUT="$ROOT/tools/upstream/openshell-$PINNED_VERSION"
if [[ -d "$CHECKOUT/.git" ]] && [[ "$(git -C "$CHECKOUT" rev-parse HEAD)" != "$PIN" ]]; then
	echo "OpenShell reference checkout differs from $PIN" >&2
	exit 1
fi

# Keep source snapshots beside Go tests so standalone module tests do not
# depend on the repository-level upstream checkout being copied into a temp dir.
for BACKEND in docker podman; do
	UPSTREAM_FIXTURE="$CHECKOUT/e2e/configs/gateway/$BACKEND.toml"
	TEST_FIXTURE="$ROOT/quarshell-gateway/internal/gatewayconfig/testdata/openshell-$PINNED_VERSION/$BACKEND.toml"
	if [[ ! -f "$UPSTREAM_FIXTURE" || ! -f "$TEST_FIXTURE" ]] || ! cmp -s "$UPSTREAM_FIXTURE" "$TEST_FIXTURE"; then
		echo "gateway config fixture snapshot differs from pinned $PINNED_VERSION source: $BACKEND.toml" >&2
		exit 1
	fi
done

FOUND=0

for MOD_DIR in "$ROOT"/quarshell-*; do
	MOD_FILE="$MOD_DIR/go.mod"
	[[ -f "$MOD_FILE" ]] || continue
	VERSION="$(awk '$1 == "github.com/NVIDIA/OpenShell/sdk/go" { print $2; exit }' "$MOD_FILE")"
	[[ -n "$VERSION" ]] || continue
	FOUND=1
	if [[ "$VERSION" != *-"$EXPECTED" ]]; then
		echo "$MOD_FILE pins OpenShell SDK $VERSION; expected commit prefix $EXPECTED" >&2
		exit 1
	fi
done

if [[ "$FOUND" != 1 ]]; then
	echo "no product module depends on the pinned OpenShell Go SDK" >&2
	exit 1
fi

echo "OpenShell SDK module versions match $PINNED_VERSION commit $PIN"
