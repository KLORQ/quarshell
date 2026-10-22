#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.0.0-dev}"
OUT="${OUT:-$ROOT/release-packages}"
mkdir -p "$OUT"
if [[ ! -x "$ROOT/release-dry-run/quarshell-gateway-linux-amd64" ]]; then
  VERSION="$VERSION" bash "$ROOT/tools/release-dry-run.sh"
fi
for arch in amd64 arm64; do
  config="$(mktemp)"
  sed -e "s/__ARCH__/$arch/g" -e "s/__VERSION__/$VERSION/g" -e "s#__ROOT__#$ROOT#g" \
    "$ROOT/packaging/nfpm.yaml.tmpl" > "$config"
  nfpm package --config "$config" --packager deb --target "$OUT/"
  nfpm package --config "$config" --packager rpm --target "$OUT/"
  rm -f "$config"
done
ls -1 "$OUT"
