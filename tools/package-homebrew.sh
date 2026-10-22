#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.0.0-dev}"
OUT="${OUT:-$ROOT/release-packages}"
mkdir -p "$OUT"
arm="$ROOT/release-dry-run/quarshell_${VERSION}_darwin_arm64.tar.gz"
intel="$ROOT/release-dry-run/quarshell_${VERSION}_darwin_amd64.tar.gz"
[[ -s "$arm" && -s "$intel" ]] || { echo "darwin archives missing; run tools/release-dry-run.sh" >&2; exit 1; }
sha_arm="$(shasum -a 256 "$arm" | awk '{print $1}')"
sha_intel="$(shasum -a 256 "$intel" | awk '{print $1}')"
sed -e "s/__VERSION__/$VERSION/g" -e "s/__SHA_DARWIN_ARM64__/$sha_arm/g" -e "s/__SHA_DARWIN_AMD64__/$sha_intel/g" \
  "$ROOT/packaging/quarshell.rb.tmpl" > "$OUT/quarshell.rb"
echo "formula: $OUT/quarshell.rb"
