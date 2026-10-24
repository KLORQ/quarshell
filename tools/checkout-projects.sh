#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
requested=("$@")

while read -r name repository commit; do
  [[ -z "${name:-}" || "$name" == \#* ]] && continue
  if ((${#requested[@]})); then
    selected=0
    for candidate in "${requested[@]}"; do
      [[ "$candidate" == "$name" ]] && selected=1
    done
    ((selected)) || continue
  fi
  destination="$root/$name"
  url="https://github.com/$repository.git"
  new_checkout=0
  if [[ -e "$destination" && ! -d "$destination/.git" ]]; then
    echo "refusing to replace non-Git path: $destination" >&2
    exit 1
  fi
  if [[ ! -d "$destination/.git" ]]; then
    git clone --no-checkout "$url" "$destination"
    new_checkout=1
  elif [[ "$(git -C "$destination" remote get-url origin)" != "$url" ]]; then
    echo "unexpected origin for $name: $(git -C "$destination" remote get-url origin)" >&2
    exit 1
  fi
  if (( ! new_checkout )) && [[ -n "$(git -C "$destination" status --porcelain)" ]]; then
    echo "refusing to change dirty checkout: $name" >&2
    exit 1
  fi
  git -C "$destination" fetch --depth 1 origin "$commit"
  git -C "$destination" checkout --detach "$commit"
  echo "$name: $commit"
done < "$root/tools/projects.lock"

if ((${#requested[@]})); then
  for candidate in "${requested[@]}"; do
    if ! awk -v name="$candidate" 'NF >= 3 && $1 !~ /^#/ && $1 == name { found=1 } END { exit !found }' "$root/tools/projects.lock"; then
      printf 'unknown project: %s\n' "$candidate" >&2
      exit 1
    fi
  done
fi
