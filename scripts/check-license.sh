#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 the quarshell authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

check_apache_repo() {
  local repo="$1"
  local notice_required="${2:-true}"
  test -f "$repo/LICENSE" || { echo "missing Apache LICENSE: $repo" >&2; exit 1; }
  test ! -e "$repo/LICENSE-MIT" || { echo "legacy MIT license is still packaged: $repo" >&2; exit 1; }
  grep -q 'Apache License' "$repo/LICENSE" || { echo "not Apache-2.0: $repo" >&2; exit 1; }
  if [[ "$notice_required" == true ]]; then
    test -f "$repo/NOTICE" || { echo "missing NOTICE: $repo" >&2; exit 1; }
    grep -q '^Copyright 2026 quarshell contributors$' "$repo/NOTICE" || {
      echo "project copyright attribution missing from NOTICE: $repo" >&2
      exit 1
    }
  fi
}

module_count=0
while read -r module revision; do
  [[ -z "${module:-}" || "$module" == \#* ]] && continue
  check_apache_repo "$module"
  ((module_count += 1))
done < tools/modules.lock

# Optional non-Go repositories are checked when explicitly checked out beside
# the Go modules. Their versions are managed separately from modules.lock.
for repo in quarshell-python quarshell-docs; do
  if [[ -d "$repo" ]]; then
    check_apache_repo "$repo"
  fi
done
if [[ -d quarshell-ui ]]; then check_apache_repo quarshell-ui false; fi

grep -q '^MIT License$' quarshell-core/slogx/LICENSE || {
  echo "quarshell-core/slogx must retain its MIT terms" >&2
  exit 1
}
grep -q '^MIT — see \[LICENSE\](LICENSE)\.$' quarshell-core/slogx/README.md || {
  echo "quarshell-core/slogx README must identify the MIT license" >&2
  exit 1
}

if rg -n 'SPDX-License-Identifier:[[:space:]]*MIT' \
  --glob '!quarshell-core/slogx/**' \
  --glob '!tools/upstream/openshell/**' \
  --glob '!tools/upstream/openshell-v0.1.4/**' \
  --glob '!tools/upstream/openshell-unpinned-*/**' \
  .; then
  echo 'active MIT SPDX identifier found outside the pinned upstream mirror' >&2
  exit 1
fi

echo "Apache-2.0 verified for $module_count locked Go modules; core/slogx retains MIT terms."
