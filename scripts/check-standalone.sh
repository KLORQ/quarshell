#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 the quarshell authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

modules=(
  quarshell-proto quarshell-cli quarshell-core quarshell-driver quarshell-driver-vm
  quarshell-gateway quarshell-policy quarshell-proxy quarshell-runtime
  quarshell-sdk
)

# Keep every module outside the workspace and wire sibling modules explicitly.
# This preserves the standalone consumer check even before the alpha tags are
# published to the public module proxy.
for module in "${modules[@]}"; do
  cp -a "$module" "$work/$module"
done

for module in "${modules[@]}"; do
  echo "== standalone $module =="
  for dependency in "${modules[@]}"; do
    [[ "$module" == "$dependency" ]] && continue
    module_path="$(sed -n 's/^module[[:space:]][[:space:]]*//p' "$dependency/go.mod" | head -1)"
    [[ -n "$module_path" ]] || continue
    go -C "$work/$module" mod edit -replace "$module_path=$work/$dependency"
  done
  GOWORK=off CGO_ENABLED=0 go -C "$work/$module" test -mod=mod ./...
done

echo "standalone GOWORK=off: PASS"
