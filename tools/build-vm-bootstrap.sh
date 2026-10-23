#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
D="${CAUTEM_VM_BUILD_DIR:-$ROOT/quarshell-driver/dist/vm-live-darwin-arm64}"
OUT="${CAUTEM_VM_BOOTSTRAP_DISK:-$D/cautem-vm-bootstrap.ext4}"
SIZE="${CAUTEM_VM_BOOTSTRAP_SIZE:-256M}"
for file in "$D/alpine-rootfs.tar" "$D/cautem-vm-guest-agent-linux-arm64" "$D/cautem-vm-umoci-linux-arm64"; do
  [[ -s "$file" ]] || { echo "missing VM runtime asset: $file (run task vm:live-darwin first)" >&2; exit 1; }
done
if command -v brew >/dev/null 2>&1; then
  E2FS_PREFIX="$(brew --prefix e2fsprogs 2>/dev/null || true)"
  if [[ -n "$E2FS_PREFIX" ]]; then
    export PATH="$E2FS_PREFIX/sbin:$E2FS_PREFIX/bin:$PATH"
  fi
fi
GOBIN="$(mktemp -d)"
trap 'rm -rf "$GOBIN"' EXIT
go -C "$ROOT/quarshell-driver-vm" build -o "$GOBIN/cautem-vm-bootstrap" ./cmd/cautem-vm-bootstrap
"$GOBIN/cautem-vm-bootstrap" --out "$OUT" --base-rootfs "$D/alpine-rootfs.tar" \
  --guest-agent "$D/cautem-vm-guest-agent-linux-arm64" \
  --umoci "$D/cautem-vm-umoci-linux-arm64" --size "$SIZE"
chmod 0444 "$OUT"
echo "$OUT"
