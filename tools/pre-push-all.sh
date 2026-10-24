#!/usr/bin/env bash
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if (($#)); then
  modules=("$@")
else
  modules=()
  while read -r module revision; do
    [[ -z "${module:-}" || "$module" == \#* ]] && continue
    modules+=("$module")
  done < "$root/tools/modules.lock"
fi
if ((${#modules[@]} == 0)); then
  echo "tools/modules.lock contains no Go modules" >&2
  exit 1
fi

status=0
for module in "${modules[@]}"; do
  if ! awk -v module="$module" 'NF >= 2 && $1 !~ /^#/ && $1 == module { found=1 } END { exit !found }' "$root/tools/modules.lock"; then
    printf 'pre-push: module is not registered in tools/modules.lock: %s\n' "$module" >&2
    status=1
    continue
  fi
  path="$root/$module/scripts/pre-push.sh"
  if [[ ! -x "$path" ]]; then
    printf '\n== %s (Go test fallback) ==\n' "$module"
    if ! GOWORK="$root/go.work" CGO_ENABLED=0 go test -C "$root/$module" ./...; then
      printf 'pre-push: FAILED: %s\n' "$module" >&2
      status=1
    fi
    continue
  fi
  printf '\n== %s ==\n' "$module"
  if ! "$path"; then
    printf 'pre-push: FAILED: %s\n' "$module" >&2
    status=1
  fi
done
if ((status)); then
  exit 1
fi
printf '\npre-push: all selected Go modules PASS\n'
