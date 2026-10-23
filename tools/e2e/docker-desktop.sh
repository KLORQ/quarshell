#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
host_os=$(go env GOOS)
host_arch=$(go env GOARCH)
if [ "$host_os/$host_arch" != darwin/arm64 ]; then
	echo "Docker Desktop acceptance requires native macOS/arm64; found $host_os/$host_arch" >&2
	exit 2
fi

# Docker Desktop exposes paths under the checkout through host file sharing.
# Keep the test workspace under the same shared root so the runner container
# and the Docker Engine receive an identical absolute bind source.
TMPDIR="$E2E_ROOT/.e2e-tmp"
mkdir -p "$TMPDIR"
export TMPDIR
source "$SCRIPT_DIR/lib.sh"

cleanup() {
	local status=$?
	e2e_remove_host_image
	e2e_cleanup_tmp || true
	return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

command -v docker >/dev/null 2>&1 || { echo "docker CLI is required" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker Desktop daemon is unavailable" >&2; exit 1; }
e2e_build_artifacts
e2e_build_sandbox_image
mkdir -p "$E2E_TMP/workspace"
chmod 755 "$E2E_TMP/docker.test"

IMAGE="${QUARSHELL_DOCKER_DESKTOP_RUNNER_IMAGE:-docker:28.0-cli}"
echo "Running workspace bind and persistence acceptance against the Docker Desktop daemon ($IMAGE)…"
e2e_run_required_test docker-desktop-workspace TestDockerDesktopWorkspaceBindE2E docker run --rm \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v "$E2E_TMP:$E2E_TMP" \
	-e DOCKER_HOST=unix:///var/run/docker.sock \
	-e QUARSHELL_DOCKER_DESKTOP_E2E=1 \
	-e "QUARSHELL_DOCKER_WORKSPACE=$E2E_TMP/workspace" \
	-e "QUARSHELL_E2E_SANDBOX_IMAGE=$E2E_SANDBOX_IMAGE" \
	"$IMAGE" "$E2E_TMP/docker.test" -test.run '^TestDockerDesktopWorkspaceBindE2E$' -test.count=1 -test.v

echo "Running host-route isolation acceptance from macOS against the Docker Desktop daemon…"
(
	cd "$E2E_ROOT/quarshell-driver"
	DOCKER_HOST=unix:///var/run/docker.sock \
	QUARSHELL_DOCKER_DESKTOP_HOST_BOUNDARY_E2E=1 \
	QUARSHELL_DOCKER_WORKSPACE="$E2E_TMP/workspace" \
	QUARSHELL_HELPERS_DIR="$E2E_HELPERS" \
	QUARSHELL_PROXY_IMAGE="$E2E_PROXY_IMAGE" \
	QUARSHELL_E2E_SANDBOX_IMAGE="$E2E_SANDBOX_IMAGE" \
	go test ./tests/e2e/docker -run '^TestDockerDesktopHostGatewayIsolationE2E$' -count=1 -v
)
