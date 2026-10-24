#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
locked_modules=0
while read -r module revision; do
  module="${module%$'\r'}"
  revision="${revision%$'\r'}"
  [[ -z "$module" || "$module" == \#* ]] && continue
  [[ "$module" =~ ^quarshell-[a-z]+(-[a-z]+)*$ ]] || { echo "invalid module: $module" >&2; exit 1; }
  [[ "$revision" =~ ^v0\.[0-9]+\.[0-9]+$ ]] || { echo "invalid pre-stable version: $revision" >&2; exit 1; }
  ((locked_modules += 1))
  if [[ ! -d "$root/$module/.git" ]]; then
    git init -q "$root/$module"
    git -C "$root/$module" remote add origin "https://github.com/KLORQ/${module}.git"
    git -C "$root/$module" fetch -q --depth 1 origin "refs/tags/$revision:refs/tags/$revision"
    git -C "$root/$module" checkout -q --detach "$revision"
  fi
  actual="$(git -C "$root/$module" describe --tags --exact-match HEAD 2>/dev/null || true)"
  [[ "$actual" == "$revision" ]] || { echo "$module: expected $revision, got $actual" >&2; exit 1; }
done < "$root/tools/modules.lock"
((locked_modules > 0)) || {
	echo "tools/modules.lock has no published module versions; populate it before checking out a clean workspace" >&2
	exit 1
}
