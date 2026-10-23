#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# Docker 28.0 is the minimum validated Engine baseline; tmpfs keeps the
# disposable daemon from accumulating anonymous volumes or exhausting the
# Desktop-backed writable layer. Override only for a daemon with the same
# network capability contract.
IMAGE="${QUARSHELL_DOCKER_E2E_IMAGE:-docker:28.0-dind}"
E2E_OWNER="$(basename "$E2E_TMP")"
E2E_INNER_TMP=/e2e
NAME="quarshell-docker-e2e-$E2E_OWNER"
E2E_CONTAINER_MEMORY="${QUARSHELL_E2E_CONTAINER_MEMORY:-4g}"
E2E_CONTAINER_CPUS="${QUARSHELL_E2E_CONTAINER_CPUS:-2}"
E2E_CONTAINER_PIDS="${QUARSHELL_E2E_CONTAINER_PIDS:-1024}"
cleanup() {
	local status=$?
	local owner=""
	if command -v docker >/dev/null 2>&1; then
		owner="$(docker inspect -f '{{index .Config.Labels "quarshell.e2e.owner"}}' "$NAME" 2>/dev/null || true)"
	fi
	if [[ "$owner" == "$E2E_OWNER" ]]; then
		if [[ "$status" != "0" ]]; then
			docker logs "$NAME" >&2 || true
		fi
		if [[ "${E2E_KEEP_CONTAINER:-0}" == "1" ]]; then
			echo "Keeping failed E2E container $NAME for diagnostics" >&2
		else
			docker rm -f "$NAME" >/dev/null 2>&1 || true
		fi
	fi
	e2e_remove_host_image
	e2e_cleanup_tmp || true
	return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

command -v docker >/dev/null || { echo "docker CLI is required" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker daemon is unavailable" >&2; exit 1; }
e2e_build_artifacts
e2e_build_sandbox_image
mkdir -p "$E2E_TMP/workspace"
# Normalize helper modes before copying them into the disposable daemon.
# bits explicitly; otherwise Docker Desktop can preserve a non-executable host
# mode and the sandbox fails with a misleading exit 137/permission denied.
chmod 755 "$E2E_TMP"/*.test "$E2E_HELPERS"/*

echo "Starting isolated Docker-in-Docker daemon ($IMAGE)…"
docker run -d --privileged --name "$NAME" \
	--label "quarshell.e2e.owner=$E2E_OWNER" \
	--memory "$E2E_CONTAINER_MEMORY" --cpus "$E2E_CONTAINER_CPUS" --pids-limit "$E2E_CONTAINER_PIDS" \
	-e DOCKER_TLS_CERTDIR= \
	--tmpfs /var/lib/docker:rw,exec,size=4g \
	-v "$E2E_ROOT:$E2E_ROOT" \
	"$IMAGE" dockerd --host=unix:///var/run/docker.sock --storage-driver=vfs --ipv6 --fixed-cidr-v6=fd00:ca7e:1::/64 >/dev/null

ready=0
for _ in $(seq 1 90); do
	if docker exec "$NAME" docker info >/dev/null 2>&1; then
		ready=1
		break
	fi
	sleep 1
done
if [[ "$ready" != "1" ]]; then
	echo "Docker-in-Docker daemon did not become ready" >&2
	exit 1
fi
echo "Installing the glibc compatibility loader for Linux test binaries…"
docker exec "$NAME" apk add --no-cache gcompat iproute2

docker exec "$NAME" mkdir -p "$E2E_INNER_TMP"
docker cp "$E2E_TMP/." "$NAME:$E2E_INNER_TMP/"
echo "Loading the sandbox and proxy fixtures into the disposable Docker daemon…"
docker exec "$NAME" docker load -i "$E2E_INNER_TMP/sandbox-image.tar"

echo "Checking nested helper execution through its container-local bind mount…"
docker exec "$NAME" docker run --rm --network none \
	-v "$E2E_INNER_TMP/helpers:/quarshell:ro" \
	--entrypoint /quarshell/quarshell-supervisor \
	"$E2E_SANDBOX_IMAGE" -- /bin/true

echo "Running Docker driver create → start → inspect → exec → stop → delete E2E…"
e2e_run_required_test docker-driver TestDockerEngineE2E docker exec \
	-e DOCKER_HOST=unix:///var/run/docker.sock \
	-e QUARSHELL_DOCKER_E2E=1 \
	-e "QUARSHELL_E2E_SANDBOX_IMAGE=$E2E_SANDBOX_IMAGE" \
	-e "QUARSHELL_PROXY_IMAGE=$E2E_PROXY_IMAGE" \
	-e "QUARSHELL_HELPERS_DIR=$E2E_INNER_TMP/helpers" \
	-e "QUARSHELL_DOCKER_WORKSPACE=$E2E_INNER_TMP/workspace" \
	"$NAME" "$E2E_INNER_TMP/docker.test" -test.run '^TestDockerEngineE2E$' -test.count=1 -test.v
