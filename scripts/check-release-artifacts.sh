#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 the quarshell authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

command -v docker >/dev/null 2>&1 || {
  echo "release artifacts: missing required tool: docker" >&2
  exit 1
}
docker info >/dev/null 2>&1 || {
  echo "release artifacts: Docker daemon is unavailable" >&2
  exit 1
}

goreleaser_image="${QUARSHELL_GORELEASER_IMAGE:-goreleaser/goreleaser:v2.12.7@sha256:a2a47c0dda85f8d40eaaa5b9765bf76c69addb6060666f8a51441410d9b008e9}"
syft_image="${QUARSHELL_SYFT_IMAGE:-anchore/syft@sha256:0356562f495d432056237fbea5cbc2d4839c9c75cd500784a66de2e7cc95ca7c}"
sbom_version="${QUARSHELL_RELEASE_VERSION:-0.0.0-dev}"

run_goreleaser() {
  if command -v goreleaser >/dev/null 2>&1; then
    goreleaser "$@"
    return
  fi
  docker run --rm \
    -v "$root:/src" \
    -w /src/quarshell-cli \
    "$goreleaser_image" "$@"
}

run_syft() {
  if command -v syft >/dev/null 2>&1; then
    syft "$@"
    return
  fi
  docker run --rm \
    -v "$root:/src:ro" \
    -v "$work:/out" \
    "$syft_image" "$@"
}

bash scripts/check-license.sh
bash scripts/check-image-pins.sh
run_goreleaser check --config .goreleaser.yml

work="$(mktemp -d)"
image="quarshell-gateway:artifact-check-$$"
cleanup() {
  docker image rm "$image" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

echo "== build release archive inputs =="
mkdir -p "$work/archive"
CGO_ENABLED=0 go build -C quarshell-cli -o "$work/archive/quarshell" ./cmd/quarshell
CGO_ENABLED=0 go build -C quarshell-gateway -o "$work/archive/quarshell-gateway" ./cmd/quarshell-gateway
cp LICENSE NOTICE "$work/archive/"
tar -C "$work" -czf "$work/quarshell-linux-amd64.tar.gz" archive
tar -tzf "$work/quarshell-linux-amd64.tar.gz" | grep -qx 'archive/LICENSE'
tar -tzf "$work/quarshell-linux-amd64.tar.gz" | grep -qx 'archive/NOTICE'

echo "== build gateway image =="
docker build --pull=false -f quarshell-gateway/Dockerfile -t "$image" .
docker run --rm --entrypoint sh "$image" -ceu '
  for file in \
    /usr/share/licenses/quarshell-gateway/LICENSE \
    /usr/share/licenses/quarshell-gateway/NOTICE \
    /usr/share/licenses/quarshell-cli/LICENSE \
    /usr/share/licenses/quarshell-cli/NOTICE \
    /usr/share/licenses/quarshell-runtime/LICENSE \
    /usr/share/licenses/quarshell-runtime/NOTICE; do
    test -s "$file"
  done
'

echo "== generate SBOM =="
run_syft "dir:/src" --source-name quarshell --source-version "$sbom_version" -o cyclonedx-json=/out/quarshell.sbom.json
test -s "$work/quarshell.sbom.json"
python3 - "$work/quarshell.sbom.json" "$sbom_version" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as sbom_file:
    component = json.load(sbom_file).get("metadata", {}).get("component", {})
expected = {"name": "quarshell", "version": sys.argv[2]}
actual = {key: component.get(key) for key in expected}
if actual != expected:
    raise SystemExit(f"SBOM source identity {actual!r} does not match {expected!r}")
PY

echo "release artifacts: PASS"
