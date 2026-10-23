#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOST_OS="$(go env GOOS)"
HOST_ARCH="$(go env GOARCH)"
D="${QUARSHELL_VM_BUILD_DIR:-$ROOT/quarshell-driver/dist/vm-live-${HOST_OS}-${HOST_ARCH}}"
OUT="${QUARSHELL_VM_BOOTSTRAP_DISK:-$D/quarshell-vm-bootstrap.ext4}"
SIZE="${QUARSHELL_VM_BOOTSTRAP_SIZE:-256M}"
for file in "$D/alpine-rootfs.tar" "$D/quarshell-vm-guest-agent-linux-$HOST_ARCH" "$D/quarshell-vm-umoci-linux-$HOST_ARCH"; do
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
go -C "$ROOT/quarshell-driver-vm" build -o "$GOBIN/quarshell-vm-bootstrap" ./cmd/quarshell-vm-bootstrap
"$GOBIN/quarshell-vm-bootstrap" --out "$OUT" --base-rootfs "$D/alpine-rootfs.tar" \
  --guest-agent "$D/quarshell-vm-guest-agent-linux-$HOST_ARCH" \
  --umoci "$D/quarshell-vm-umoci-linux-$HOST_ARCH" --size "$SIZE"
chmod 0444 "$OUT"
echo "$OUT"
