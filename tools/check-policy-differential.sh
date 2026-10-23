#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_FILE="$ROOT/tools/upstream/UPSTREAM_VERSION"
if [[ ! -f "$VERSION_FILE" ]]; then
	echo "missing pinned OpenShell version: $VERSION_FILE" >&2
	exit 1
fi
PINNED_VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
CHECKOUT="$ROOT/tools/upstream/openshell-$PINNED_VERSION"
if [[ ! -f "$CHECKOUT/crates/openshell-policy/Cargo.toml" ]]; then
	echo "pinned upstream policy is missing: $CHECKOUT" >&2
	exit 1
fi
if ! command -v cargo >/dev/null 2>&1; then
	echo "cargo is required for the upstream policy differential" >&2
	exit 1
fi

ORACLE_DIR="$ROOT/tools/policy-oracle"
ORACLE="$ORACLE_DIR/target/release/policy-oracle"
VALIDATE="$ORACLE_DIR/target/policy-validate"
cargo build --locked --release --manifest-path "$ORACLE_DIR/Cargo.toml"
go build -o "$VALIDATE" github.com/KLORQ/quarshell-policy/cmd/quarshell-policy-validate

go run "$ROOT"/tools/cmd/quarshell-tools/*.go conformance --candidate-cmd "go run github.com/KLORQ/quarshell-policy/cmd/quarshell-policy-check" --oracle-cmd "$ORACLE"
go run "$ROOT"/tools/cmd/quarshell-tools/*.go conformance --candidate-cmd "$VALIDATE" --oracle-cmd "$ORACLE --validate"
