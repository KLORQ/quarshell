#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KIND="${KIND:-$HOME/go/bin/kind}"
CLUSTER="${QUARSHELL_KIND_CLUSTER:-quarshell-kind-test}"
KUBECONFIG_FILE="$(mktemp "${TMPDIR:-/tmp}/quarshell-kind-kubeconfig.XXXXXX")"
trap 'rm -f "$KUBECONFIG_FILE"' EXIT

if [[ ! -x "$KIND" ]]; then
	echo "kind is not installed at $KIND" >&2
	exit 1
fi
"$KIND" get kubeconfig --name "$CLUSTER" > "$KUBECONFIG_FILE"
QUARSHELL_KUBECONFIG="$KUBECONFIG_FILE" QUARSHELL_K8S_NAMESPACE="${QUARSHELL_K8S_NAMESPACE:-default}" \
	go -C "$ROOT/quarshell-gateway" test ./credentialstore -run TestKubernetesStoreIntegrationLive -count=1
