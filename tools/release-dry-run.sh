#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT/release-dry-run}"
VERSION="${VERSION:-0.0.0-dev}"
rm -rf "$OUT"
mkdir -p "$OUT"

TARGETS=(
	"darwin arm64"
	"darwin amd64"
	"linux amd64"
	"linux arm64"
	"windows amd64"
)

build() {
	local os="$1" arch="$2" module="$3" package="$4" name="$5"
	local extension=""
	if [[ "$os" == "windows" ]]; then
		extension=".exe"
	fi
	local output="$OUT/${name}-${os}-${arch}${extension}"
	echo "building $name $os/$arch"
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go -C "$ROOT/$module" build -trimpath -o "$output" "$package"
	{
		echo "### $name $os/$arch"
		go version -m "$output" 2>/dev/null || true
		echo
	} >> "$OUT/SBOM.txt"
}

for target in "${TARGETS[@]}"; do
	read -r os arch <<< "$target"
	build "$os" "$arch" quarshell-gateway ./cmd/quarshell-gateway quarshell-gateway
	build "$os" "$arch" quarshell-cli ./cmd/quarshell quarshell-cli
	build "$os" "$arch" quarshell-policy ./cmd/prover quarshell-prover
done

for target in "${TARGETS[@]}"; do
	read -r os arch <<< "$target"
	extension=""
	[[ "$os" == "windows" ]] && extension=".exe"
	archive="$OUT/quarshell_${VERSION}_${os}_${arch}.tar.gz"
	tar -czf "$archive" -C "$OUT" \
		"quarshell-cli-${os}-${arch}${extension}" \
		"quarshell-gateway-${os}-${arch}${extension}" \
		"quarshell-prover-${os}-${arch}${extension}"
done
( cd "$OUT" && shasum -a 256 quarshell-* quarshell_*.tar.gz > SHA256SUMS )
go run "$ROOT"/tools/cmd/quarshell-tools/*.go release-manifest "$OUT"
echo "release dry-run: $OUT"
