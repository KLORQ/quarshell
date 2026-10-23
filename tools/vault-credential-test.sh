#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAME="quarshell-vault-test"
PORT="${QUARSHELL_VAULT_PORT:-58200}"

cleanup() {
	docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
docker run --rm -d --name "$NAME" \
	-e VAULT_DEV_ROOT_TOKEN_ID=test \
	-e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
	-p "127.0.0.1:${PORT}:8200" hashicorp/vault:1.17 >/dev/null

for _ in $(seq 1 60); do
	if curl -sf "http://127.0.0.1:${PORT}/v1/sys/health" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
if ! curl -sf "http://127.0.0.1:${PORT}/v1/sys/health" >/dev/null 2>&1; then
	echo "vault did not become ready" >&2
	exit 1
fi

QUARSHELL_VAULT_ADDR="http://127.0.0.1:${PORT}" QUARSHELL_VAULT_TOKEN=test \
	go -C "$ROOT/quarshell-gateway" test ./credentialstore -run TestVaultStoreIntegrationLive -count=1
