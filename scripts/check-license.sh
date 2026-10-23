#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 the quarshell authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

apache_modules=(
  quarshell-cli quarshell-core quarshell-docs
  quarshell-driver quarshell-gateway quarshell-policy quarshell-proxy
  quarshell-runtime quarshell-sdk quarshell-python
)

for module in "${apache_modules[@]}"; do
  test -f "$module/LICENSE" || { echo "missing Apache LICENSE: $module" >&2; exit 1; }
  cmp -s LICENSE "$module/LICENSE" || { echo "LICENSE differs from root: $module" >&2; exit 1; }
  test -f "$module/NOTICE" || { echo "missing NOTICE: $module" >&2; exit 1; }
  test ! -e "$module/LICENSE-MIT" || { echo "legacy MIT license is still packaged: $module" >&2; exit 1; }
  grep -q 'Apache License' "$module/LICENSE" || { echo "not Apache-2.0: $module" >&2; exit 1; }
  grep -q '^Copyright 2026 quarshell contributors$' "$module/NOTICE" || {
    echo "project copyright attribution missing from NOTICE: $module" >&2
    exit 1
  }
done

grep -q '^MIT License$' quarshell-core/slogx/LICENSE || {
  echo "quarshell-core/slogx must retain its MIT license" >&2
  exit 1
}
grep -q '^MIT — see \[LICENSE\](LICENSE)\.$' quarshell-core/slogx/README.md || {
  echo "quarshell-core/slogx README must identify the MIT license" >&2
  exit 1
}

grep -q 'license = "Apache-2.0"' quarshell-python/pyproject.toml
grep -q 'license-files = \["LICENSE", "NOTICE"\]' quarshell-python/pyproject.toml
grep -q 'org.opencontainers.image.licenses="Apache-2.0"' quarshell-gateway/Dockerfile

if rg -n 'SPDX-License-Identifier:[[:space:]]*MIT' \
  --glob '!quarshell-core/slogx/**' \
  --glob '!tools/upstream/openshell/**' \
  --glob '!tools/upstream/openshell-v0.1.4/**' \
  --glob '!tools/upstream/openshell-unpinned-*/**' \
  .; then
  echo 'active MIT SPDX identifier found outside the pinned upstream mirror' >&2
  exit 1
fi

if rg -n -i 'MIT License|license[^[:alnum:]]+MIT' \
  --glob '!scripts/check-license.sh' \
  --glob '!README.md' \
  --glob '!**/README.md' \
  --glob '!NOTICE' \
  --glob '!**/NOTICE' \
  --glob '!plans/**' \
  --glob '!tools/**' \
  --glob '!quarshell-core/slogx/**' \
  --glob '!tools/upstream/openshell/**' \
  --glob '!tools/upstream/openshell-v0.1.4/**' \
  --glob '!tools/upstream/openshell-unpinned-*/**' \
  --glob '!**/package-lock.json' \
  --glob '!**/LICENSE' .; then
  echo 'MIT license reference found outside the pinned upstream mirror' >&2
  exit 1
fi

echo "Apache-2.0 license verified for ${#apache_modules[@]} modules; core/slogx retains MIT terms."
