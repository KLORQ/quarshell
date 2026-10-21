#!/usr/bin/env bash
# Verify each checkout as an external Go consumer, using the versions in go.mod.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
modules=( $(bash "$root/tools/module-list.sh") )
failed=0
for module in "${modules[@]}"; do
  echo "== published dependencies $module =="
  if awk '/^[[:space:]]*replace([[:space:]]|\()/ { found=1 } END { exit !found }' "$root/$module/go.mod"; then
    echo "$module/go.mod contains a replace directive; published dependency gate requires released module versions" >&2
    failed=1
    continue
  fi
  if ! (cd "$root/$module" && GOWORK=off CGO_ENABLED=0 go test -mod=readonly ./...); then
    failed=1
  fi
done
if [[ "$failed" != 0 ]]; then
  echo "published dependency gate: FAIL; update and publish dependency versions before the consumer release" >&2
  exit 1
fi
echo "published dependency gate: PASS"
