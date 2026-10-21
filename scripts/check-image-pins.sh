#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

fail=0
require_pin() {
  local file="$1" pattern="$2" label="$3"
  if ! grep -Eq "$pattern" "$file"; then
    echo "image pins: $label is not pinned by sha256 in $file" >&2
    fail=1
  fi
}

require_pin quarshell-gateway/Dockerfile '^FROM golang:1\.27-bookworm@sha256:[0-9a-f]{64} AS build$' 'gateway build base'
require_pin quarshell-driver/scripts/run-live-darwin-arm64-acceptance.sh 'docker\.io/library/alpine:3\.22@sha256:[0-9a-f]{64}' 'Darwin VM acceptance rootfs base'
require_pin quarshell-driver/scripts/run-live-kvm-acceptance.sh 'docker\.io/library/alpine:3\.22@sha256:[0-9a-f]{64}' 'KVM VM acceptance rootfs base'
require_pin quarshell-gateway/Dockerfile '^FROM debian:bookworm-slim@sha256:[0-9a-f]{64}$' 'gateway runtime base'
require_pin quarshell-ui/Dockerfile '^FROM node:22-bookworm-slim@sha256:[0-9a-f]{64} AS build$' 'console build base'
require_pin quarshell-ui/Dockerfile '^FROM nginx:1\.27-alpine@sha256:[0-9a-f]{64}$' 'console runtime base'
require_pin quarshell-gateway/compose/docker-compose.yml '^    image: mirror\.gcr\.io/library/postgres:17\.10-alpine3\.23@sha256:[0-9a-f]{64}$' 'PostgreSQL backup client'
require_pin quarshell-runtime/images/sandbox/Dockerfile '^ARG DEBIAN_BASE=debian:bookworm@sha256:[0-9a-f]{64}$' 'sandbox Debian base'
require_pin quarshell-runtime/images/sandbox/Dockerfile '^COPY --from=ghcr\.io/astral-sh/uv:0\.10\.8@sha256:[0-9a-f]{64} ' 'sandbox uv source'
require_pin scripts/check-release-artifacts.sh 'goreleaser/goreleaser:v2\.12\.7@sha256:[0-9a-f]{64}' 'GoReleaser tool image'
require_pin scripts/check-release-artifacts.sh 'anchore/syft@sha256:[0-9a-f]{64}' 'Syft tool image'

if [[ "$fail" != 0 ]]; then
  exit 1
fi
echo "image pins: PASS"
