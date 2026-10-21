#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
test -s README.md
test -s profile/README.md
test -s profile/cautem-org-avatar.png
file profile/cautem-org-avatar.png | grep -q 'PNG image data'
git diff --check
if command -v actionlint >/dev/null 2>&1 && [ -d .github/workflows ]; then
  actionlint .github/workflows/*.yml
fi
printf 'pre-push: PASS\n'
