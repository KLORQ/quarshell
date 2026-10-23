#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
go test -C "$ROOT/quarshell-proto" ./compat -run '^TestOpenShellWireGoldenMatchesGeneratedDescriptors$' -count=1
