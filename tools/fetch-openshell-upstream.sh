#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pin="$(tr -d '[:space:]' < "$root/tools/upstream/UPSTREAM_SHA")"
version="$(tr -d '[:space:]' < "$root/tools/upstream/UPSTREAM_VERSION")"
target="$root/tools/upstream/openshell-$version"

[[ "$pin" =~ ^[0-9a-f]{40}$ ]] || { echo "invalid OpenShell pin" >&2; exit 1; }
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid OpenShell version" >&2; exit 1; }
if [[ -e "$target" ]]; then
  [[ -d "$target/.git" ]] || { echo "OpenShell target exists but is not a checkout: $target" >&2; exit 1; }
  [[ "$(git -C "$target" rev-parse HEAD)" == "$pin" ]] || { echo "OpenShell checkout differs from $pin" >&2; exit 1; }
  echo "OpenShell reference checkout matches $pin"
  exit 0
fi

temporary="$(mktemp -d "$root/tools/upstream/.openshell.XXXXXX")"
trap '[[ ! -d "$temporary" ]] || rm -rf -- "$temporary"' EXIT
git -C "$temporary" init -q
git -C "$temporary" remote add origin https://github.com/NVIDIA/OpenShell.git
git -C "$temporary" fetch -q --depth 1 origin "$pin"
git -C "$temporary" checkout -q --detach FETCH_HEAD
[[ "$(git -C "$temporary" rev-parse HEAD)" == "$pin" ]] || { echo "fetched OpenShell commit differs from pin" >&2; exit 1; }
mv "$temporary" "$target"
echo "OpenShell $version reference checkout ready at $pin"
