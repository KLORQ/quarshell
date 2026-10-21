#!/usr/bin/env bash
set -euo pipefail
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
modules=(
  quarshell-cli quarshell-core quarshell-display quarshell-docs quarshell-driver
  quarshell-gateway quarshell-org-profile quarshell-proto quarshell-providers quarshell-proxy
  quarshell-python quarshell-runtime quarshell-sdk quarshell-slogx quarshell-ui
)
if (($#)); then
  modules=("$@")
fi
status=0
for module in "${modules[@]}"; do
  path="$root/$module/scripts/pre-push.sh"
  if [[ ! -x "$path" ]]; then
    printf 'pre-push: missing executable %s\n' "$path" >&2
    status=1
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
printf '\npre-push: all selected modules PASS\n'
