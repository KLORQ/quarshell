#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

module=$(sed -n 's/^module //p' go.mod | head -n 1)
[ -n "$module" ] || { echo "pre-push: go.mod has no module directive" >&2; exit 1; }
if [ -z "${GOWORK:-}" ]; then
  workspace="$(dirname "$repo_dir")/go.work"
  if [ -f "$workspace" ] && grep -Fq "./$(basename "$repo_dir")" "$workspace"; then
    export GOWORK="$workspace"
  else
    export GOWORK=off
  fi
fi
export CGO_ENABLED="${CGO_ENABLED:-0}"

printf 'pre-push: %s (GOWORK=%s, CGO_ENABLED=%s)\n' "$module" "$GOWORK" "$CGO_ENABLED"
go_files=$(find . -type f -name '*.go' -not -path './vendor/*' -print -quit)
if [ -n "$go_files" ]; then
  unformatted=$(find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -l)
  if [ -n "$unformatted" ]; then
    printf 'gofmt required:\n%s\n' "$unformatted" >&2
    exit 1
  fi
fi
tmp_tools=$(mktemp -d)
trap 'rm -rf "$tmp_tools"' EXIT HUP INT TERM
if ! command -v buf >/dev/null 2>&1; then
  GOBIN="$tmp_tools/bin" go install github.com/bufbuild/buf/cmd/buf@v1.66.0
  export PATH="$tmp_tools/bin:$PATH"
fi
./scripts/check-openshell-proto-source.sh
./scripts/check-typescript-package-notices.sh
python3 ./scripts/materialize-openshell-protos.py
npm ci --prefix typescript
buf generate --template buf.gen.openshell.yaml --path proto/openshell
buf lint
buf generate --template buf.gen.yaml
npm run generate --prefix typescript
npm run build --prefix typescript
git diff --exit-code -- . ':!scripts/pre-push.sh' ':!scripts/pre-release.sh'
go mod verify
go test -mod=readonly ./...
go vet -mod=readonly ./...
go build -mod=readonly ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run --timeout=5m ./...
printf 'pre-push: PASS\n'
