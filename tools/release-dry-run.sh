#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT/release-dry-run}"
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
	build "$os" "$arch" quarshell-cli ./cmd/cautem quarshell-cli
	build "$os" "$arch" quarshell-prover ./cmd/quarshell-prover quarshell-prover
done

( cd "$OUT" && shasum -a 256 quarshell-* > SHA256SUMS )
python3 - "$OUT" <<'PY'
import hashlib, json, sys
from pathlib import Path
out = Path(sys.argv[1])
artifacts = []
for path in sorted(out.glob("quarshell-*")):
    if path.name == "SBOM.txt":
        continue
    artifacts.append({"name": path.name, "sha256": hashlib.sha256(path.read_bytes()).hexdigest(), "size": path.stat().st_size})
(out / "manifest.json").write_text(json.dumps({"artifacts": artifacts, "signed": False}, indent=2) + "\n")
PY
echo "release dry-run: $OUT"
