#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 the quarshell authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output="$(mktemp -d "$root/.docs-check.XXXXXX")"
trap 'rm -rf "$output"' EXIT
command -v docker >/dev/null 2>&1 || {
  echo "docs strict: missing required tool: docker" >&2
  exit 1
}
docker info >/dev/null 2>&1 || {
  echo "docs strict: Docker daemon is unavailable" >&2
  exit 1
}

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -v "$root/quarshell-docs:/docs" \
  -v "$output:/out" \
  -w /docs \
  python@sha256:bf44cdfcb76cd3b41e879bc058fc37ec5872002ccfde7fcb765e218cde0cd79c \
  sh -ceu '
    python -m venv /out/venv
    . /out/venv/bin/activate
    pip install --disable-pip-version-check --no-cache-dir -r requirements.txt
    mkdocs build --strict --site-dir /out/site
    for lang in "" ru/; do
      page="/out/site/${lang}index.html"
      for meta in \
        "property=\"og:title\"" \
        "property=\"og:description\"" \
        "property=\"og:url\"" \
        "property=\"og:image\" content=\"https://klorq.github.io/sandbox.dev/assets/images/quarshell-app-icon.png\"" \
        "property=\"og:image:width\" content=\"1317\"" \
        "property=\"og:image:height\" content=\"1194\"" \
        "name=\"twitter:image\" content=\"https://klorq.github.io/sandbox.dev/assets/images/quarshell-app-icon.png\"" \
        "name=\"twitter:card\" content=\"summary_large_image\""; do
        grep -q "$meta" "$page" || { echo "missing social metadata: $meta in $page" >&2; grep -n "og:\|twitter:" "$page" >&2 || true; exit 1; }
      done
    done
    grep -q "property=\"og:title\" content=\"Quarshell — Sandboxes for coding agents\"" /out/site/index.html || { grep -n "og:title" /out/site/index.html >&2; exit 1; }
    grep -q "property=\"og:title\" content=\"Quarshell — песочницы для coding-агентов\"" /out/site/ru/index.html || { grep -n "og:title" /out/site/ru/index.html >&2; exit 1; }
    grep -q "property=\"og:locale\" content=\"en_US\"" /out/site/index.html || { grep -n "og:locale" /out/site/index.html >&2; exit 1; }
    grep -q "property=\"og:locale\" content=\"ru_RU\"" /out/site/ru/index.html || { grep -n "og:locale" /out/site/ru/index.html >&2; exit 1; }
    for lang in "" ru/; do
      page="/out/site/${lang}index.html"
      grep -q "quarshell-mascot-no-tablet.png" "$page" || { echo "hero mascot missing in $page" >&2; exit 1; }
      ! grep -q "quarshell-mascot.png" "$page" || { echo "old hero mascot still referenced in $page" >&2; exit 1; }
    done
  '

echo "docs strict: PASS"
