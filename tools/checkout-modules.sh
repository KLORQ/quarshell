#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
locked_modules=0
while read -r module revision; do
  [[ -z "$module" || "$module" == \#* ]] && continue
  [[ "$module" =~ ^quarshell-[a-z]+$ ]] || { echo "invalid module: $module" >&2; exit 1; }
  [[ "$revision" =~ ^[0-9a-f]{40}$ ]] || { echo "invalid revision: $revision" >&2; exit 1; }
  ((locked_modules += 1))
  if [[ ! -d "$root/$module/.git" ]]; then
    git init -q "$root/$module"
    git -C "$root/$module" remote add origin "https://github.com/KLORQ/${module}.git"
    git -C "$root/$module" fetch -q --depth 1 origin "$revision"
    git -C "$root/$module" checkout -q --detach FETCH_HEAD
  fi
  actual="$(git -C "$root/$module" rev-parse HEAD)"
  [[ "$actual" == "$revision" ]] || { echo "$module: expected $revision, got $actual" >&2; exit 1; }
done < "$root/tools/modules.lock"
((locked_modules > 0)) || {
	echo "tools/modules.lock has no committed source pins; publish dependency commits before checking out a clean workspace" >&2
	exit 1
}
