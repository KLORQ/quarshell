#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# macOS exposes TMPDIR through /var, while filepath.EvalSymlinks in the driver
# canonicalizes workspace bind sources to /private/var. Mount and pass that
# same physical path to the nested Podman service so it exists on both sides.
E2E_TMP="$(cd "$E2E_TMP" && pwd -P)"
E2E_HELPERS="$E2E_TMP/helpers"
E2E_FIXTURES="$E2E_TMP/fixtures"
E2E_SANDBOX_ARCHIVE="$E2E_TMP/sandbox-image.tar"

E2E_OWNER="$(basename "$E2E_TMP")"
E2E_INNER_TMP=/e2e
NAME="quarshell-podman-e2e-$E2E_OWNER"
E2E_CONTAINER_MEMORY="${QUARSHELL_E2E_CONTAINER_MEMORY:-4g}"
E2E_CONTAINER_CPUS="${QUARSHELL_E2E_CONTAINER_CPUS:-2}"
E2E_CONTAINER_PIDS="${QUARSHELL_E2E_CONTAINER_PIDS:-1024}"
# Keep account-specific subordinate ID ranges disjoint. Rootful explicit-map
# tests retain the root range at 100000; rootless lanes use the podman range.
ROOTFUL_SUBUID_START="${QUARSHELL_PODMAN_ROOTFUL_SUBUID_START:-100000}"
ROOTLESS_SUBUID_START="${QUARSHELL_PODMAN_ROOTLESS_SUBUID_START:-200000}"
CONTAINERS_SUBUID_START="${QUARSHELL_PODMAN_CONTAINERS_SUBUID_START:-300000}"
SUBUID_RANGE_SIZE="${QUARSHELL_PODMAN_SUBUID_RANGE_SIZE:-65536}"

validate_subid_value() {
	local name="$1" value="$2"
	if [[ ! "$value" =~ ^[0-9]{1,10}$ ]] || (( 10#$value > 2147418111 )); then
		echo "$name must be a decimal integer in [0, 2147418111]" >&2
		exit 1
	fi
}
validate_subid_value QUARSHELL_PODMAN_ROOTFUL_SUBUID_START "$ROOTFUL_SUBUID_START"
validate_subid_value QUARSHELL_PODMAN_ROOTLESS_SUBUID_START "$ROOTLESS_SUBUID_START"
validate_subid_value QUARSHELL_PODMAN_CONTAINERS_SUBUID_START "$CONTAINERS_SUBUID_START"
validate_subid_value QUARSHELL_PODMAN_SUBUID_RANGE_SIZE "$SUBUID_RANGE_SIZE"
ROOTFUL_SUBUID_START=$((10#$ROOTFUL_SUBUID_START))
ROOTLESS_SUBUID_START=$((10#$ROOTLESS_SUBUID_START))
CONTAINERS_SUBUID_START=$((10#$CONTAINERS_SUBUID_START))
SUBUID_RANGE_SIZE=$((10#$SUBUID_RANGE_SIZE))
if (( SUBUID_RANGE_SIZE == 0 || ROOTFUL_SUBUID_START + SUBUID_RANGE_SIZE > 2147483647 || ROOTLESS_SUBUID_START + SUBUID_RANGE_SIZE > 2147483647 || CONTAINERS_SUBUID_START + SUBUID_RANGE_SIZE > 2147483647 )); then
	echo "Podman subordinate ID ranges must be non-empty and fit in signed 32-bit IDs" >&2
	exit 1
fi
subid_ranges_overlap() {
	local left="$1" right="$2"
	(( left < right + SUBUID_RANGE_SIZE && right < left + SUBUID_RANGE_SIZE ))
}
if subid_ranges_overlap "$ROOTFUL_SUBUID_START" "$ROOTLESS_SUBUID_START" || \
	subid_ranges_overlap "$ROOTFUL_SUBUID_START" "$CONTAINERS_SUBUID_START" || \
	subid_ranges_overlap "$ROOTLESS_SUBUID_START" "$CONTAINERS_SUBUID_START"; then
	echo "rootful, rootless, and containers subordinate ID ranges must not overlap" >&2
	exit 1
fi
# Build the same Podman version used by the required rootful and rootless CI
# lanes. Overrides are accepted only when they report this exact version.
IMAGE="${QUARSHELL_PODMAN_IMAGE:-}"
PORT=""
PODMAN_IMAGE_CREATED=0
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
		docker rm -f "$NAME" >/dev/null 2>&1 || true
	fi
	if [[ "$PODMAN_IMAGE_CREATED" == "1" ]]; then
		docker image rm -f "$IMAGE" >/dev/null 2>&1 || true
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
if [[ -z "${QUARSHELL_PODMAN_IMAGE:-}" ]]; then
	IMAGE="localhost/quarshell-podman6-e2e:$E2E_OWNER"
	PODMAN_IMAGE_CREATED=1
		QUARSHELL_PODMAN_FIXTURE_DIGEST_FILE="$E2E_TMP/podman6-fixture-image-id.txt" \
			"$E2E_ROOT/quarshell-driver/scripts/build-podman6-fixture.sh" "$IMAGE"
fi
e2e_build_artifacts
e2e_build_sandbox_image
mkdir -p "$E2E_TMP/workspace"
chmod 755 "$E2E_TMP" "$E2E_TMP/workspace" "$E2E_HELPERS"
ROOTLESS_WORKSPACE="$E2E_TMP/workspace-rootless"
mkdir -p "$ROOTLESS_WORKSPACE"
chmod 0777 "$ROOTLESS_WORKSPACE"
chmod 644 "$E2E_SANDBOX_ARCHIVE"
chmod 755 "$E2E_TMP"/*.test
chmod 755 "$E2E_HELPERS"/*

docker run -d --name "$NAME" --privileged --cgroupns=host --device /dev/fuse \
	--label "quarshell.e2e.owner=$E2E_OWNER" \
	--memory "$E2E_CONTAINER_MEMORY" --cpus "$E2E_CONTAINER_CPUS" --pids-limit "$E2E_CONTAINER_PIDS" \
	--security-opt label=disable -e STORAGE_DRIVER=vfs \
	--tmpfs /var/lib/containers:rw,exec,size=4g \
	--tmpfs /home/podman/.local/share/containers:rw,exec,size=4g,uid=1000,gid=1000 \
	--tmpfs /run/user/1000:rw,exec,size=1g,mode=0700,uid=1000,gid=1000 \
	--tmpfs /tmp:rw,exec,size=2g \
	--tmpfs /var/tmp:rw,exec,size=2g \
	-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
	-v "$E2E_ROOT:$E2E_ROOT" -v "$E2E_TMP:$E2E_TMP" -p 127.0.0.1::18500 \
	-e "QUARSHELL_PODMAN_ROOTFUL_SUBUID_START=$ROOTFUL_SUBUID_START" \
	-e "QUARSHELL_PODMAN_ROOTLESS_SUBUID_START=$ROOTLESS_SUBUID_START" \
	-e "QUARSHELL_PODMAN_CONTAINERS_SUBUID_START=$CONTAINERS_SUBUID_START" \
	-e "QUARSHELL_PODMAN_SUBUID_RANGE_SIZE=$SUBUID_RANGE_SIZE" \
	"$IMAGE" \
	sh -lc 'id podman >/dev/null 2>&1 || useradd --create-home --uid 1000 podman; id containers >/dev/null 2>&1 || useradd --system --no-create-home containers; printf "root:%s:%s\npodman:%s:%s\ncontainers:%s:%s\n" "$QUARSHELL_PODMAN_ROOTFUL_SUBUID_START" "$QUARSHELL_PODMAN_SUBUID_RANGE_SIZE" "$QUARSHELL_PODMAN_ROOTLESS_SUBUID_START" "$QUARSHELL_PODMAN_SUBUID_RANGE_SIZE" "$QUARSHELL_PODMAN_CONTAINERS_SUBUID_START" "$QUARSHELL_PODMAN_SUBUID_RANGE_SIZE" > /etc/subuid; printf "root:%s:%s\npodman:%s:%s\ncontainers:%s:%s\n" "$QUARSHELL_PODMAN_ROOTFUL_SUBUID_START" "$QUARSHELL_PODMAN_SUBUID_RANGE_SIZE" "$QUARSHELL_PODMAN_ROOTLESS_SUBUID_START" "$QUARSHELL_PODMAN_SUBUID_RANGE_SIZE" "$QUARSHELL_PODMAN_CONTAINERS_SUBUID_START" "$QUARSHELL_PODMAN_SUBUID_RANGE_SIZE" > /etc/subgid; podman system service --time=0 tcp:0.0.0.0:18500' >/dev/null

docker exec "$NAME" mkdir -p "$E2E_INNER_TMP"
docker cp "$E2E_TMP/." "$NAME:$E2E_INNER_TMP/"

PORT="$(docker port "$NAME" 18500/tcp | sed 's/.*://')"
e2e_wait_http "http://127.0.0.1:$PORT/_ping" "Podman Engine API"

PODMAN_VERSION="$(docker exec "$NAME" podman version --format '{{.Version}}')"
if [[ "$PODMAN_VERSION" != "6.1.4" ]]; then
	echo "required Podman version is 6.1.4; found $PODMAN_VERSION" >&2
	exit 1
fi
echo "Podman $PODMAN_VERSION selected; all proxy and Gateway scenarios are mandatory"

echo "Loading the sandbox and proxy fixtures into disposable Podman…"
docker exec "$NAME" podman load -i "$E2E_INNER_TMP/sandbox-image.tar"

echo "Running rootful Podman user-namespace matrix without proxy dependencies…"
e2e_run_required_test podman-userns-rootful TestPodmanUsernsMatrixE2E docker exec \
	-e DOCKER_HOST=tcp://127.0.0.1:18500 \
	-e QUARSHELL_PODMAN_USERNS_E2E=1 \
	-e QUARSHELL_PODMAN_USERNS_STRICT=1 \
	-e "QUARSHELL_PODMAN_WORKSPACE=$E2E_TMP/workspace" \
	-e "QUARSHELL_E2E_SANDBOX_IMAGE=$E2E_SANDBOX_IMAGE" \
	"$NAME" "$E2E_INNER_TMP/podman.test" -test.run '^TestPodmanUsernsMatrixE2E$' -test.count=1 -test.v

cd "$E2E_ROOT/quarshell-driver"
e2e_run_required_test podman-driver-rootful TestPodmanEngineE2E env \
	CGO_ENABLED=0 \
	DOCKER_HOST="tcp://127.0.0.1:$PORT" \
	QUARSHELL_PODMAN_E2E=1 \
	QUARSHELL_PODMAN_WORKSPACE="$E2E_TMP/workspace" \
	QUARSHELL_E2E_SANDBOX_IMAGE="$E2E_SANDBOX_IMAGE" \
	QUARSHELL_PROXY_IMAGE="$E2E_PROXY_IMAGE" \
	QUARSHELL_HELPERS_DIR="$E2E_HELPERS" \
	go test ./tests/e2e/podman -run '^TestPodmanEngineE2E$' -count=1 -v

echo "Starting rootless Podman service for the full user-namespace E2E matrix…"
docker exec "$NAME" sh -lc 'mkdir -p /run/user/1000 /home/podman/.config/containers; chown -R podman:podman /home/podman; chown 1000:1000 /run/user/1000; chmod 700 /run/user/1000'
# Rootless fixture commands run as UID 1000 and create temporary bind probes.
# Keep rootless artifacts separate from root-owned rootful probe files.
docker exec --user podman -d \
	-e HOME=/home/podman \
	-e XDG_RUNTIME_DIR=/run/user/1000 \
	-e STORAGE_DRIVER=vfs \
	"$NAME" sh -lc 'while true; do podman system service --time=0 tcp:0.0.0.0:18501 >>/tmp/podman-rootless-service.log 2>&1 & pid=$!; echo "$pid" >/tmp/podman-rootless-service.pid; wait "$pid"; sleep 2; done'
ready=0
for _ in $(seq 1 90); do
	if docker exec "$NAME" curl --fail --silent http://127.0.0.1:18501/_ping >/dev/null 2>&1; then
		ready=1
		break
	fi
	sleep 1
done
if [[ "$ready" != "1" ]]; then
	docker exec "$NAME" sh -lc 'cat /tmp/podman-rootless-service.log 2>/dev/null || true; ls -ld /run/user/1000 /home/podman 2>/dev/null || true' >&2
	echo "rootless Podman API did not become ready" >&2
	exit 1
fi
docker exec --user podman -e HOME=/home/podman -e XDG_RUNTIME_DIR=/run/user/1000 \
	"$NAME" podman load -i "$E2E_INNER_TMP/sandbox-image.tar"
echo "Running rootless Podman user-namespace matrix without proxy dependencies…"
e2e_run_required_test podman-userns-rootless TestPodmanUsernsMatrixE2E docker exec --user podman \
	-e HOME=/home/podman \
	-e XDG_RUNTIME_DIR=/run/user/1000 \
	-e DOCKER_HOST=tcp://127.0.0.1:18501 \
	-e QUARSHELL_PODMAN_ROOTLESS=1 \
	-e QUARSHELL_PODMAN_USERNS_E2E=1 \
	-e QUARSHELL_PODMAN_USERNS_STRICT=1 \
	-e "QUARSHELL_PODMAN_WORKSPACE=$ROOTLESS_WORKSPACE" \
	-e "QUARSHELL_E2E_SANDBOX_IMAGE=$E2E_SANDBOX_IMAGE" \
	"$NAME" "$E2E_INNER_TMP/podman.test" -test.run '^TestPodmanUsernsMatrixE2E$' -test.count=1 -test.v
echo "Running the Podman lifecycle and userns corpus against rootless Podman…"
e2e_run_required_test podman-driver-rootless TestPodmanEngineE2E docker exec --user podman \
	-e HOME=/home/podman \
	-e XDG_RUNTIME_DIR=/run/user/1000 \
	-e DOCKER_HOST=tcp://127.0.0.1:18501 \
	-e QUARSHELL_PODMAN_E2E=1 \
	-e QUARSHELL_PODMAN_ROOTLESS=1 \
	-e "QUARSHELL_PODMAN_WORKSPACE=$ROOTLESS_WORKSPACE" \
	-e "QUARSHELL_E2E_SANDBOX_IMAGE=$E2E_SANDBOX_IMAGE" \
	-e "QUARSHELL_PROXY_IMAGE=$E2E_PROXY_IMAGE" \
	-e "QUARSHELL_HELPERS_DIR=$E2E_HELPERS" \
	"$NAME" "$E2E_INNER_TMP/podman.test" -test.run '^TestPodmanEngineE2E$' -test.count=1 -test.v

echo "Running rootless Podman daemon/service recovery E2E…"
e2e_run_required_test podman-driver-rootless-recovery TestPodmanRootlessServiceRecovery docker exec --user podman \
	-e HOME=/home/podman \
	-e XDG_RUNTIME_DIR=/run/user/1000 \
	-e DOCKER_HOST=tcp://127.0.0.1:18501 \
	-e QUARSHELL_PODMAN_ROOTLESS=1 \
	-e "QUARSHELL_PODMAN_WORKSPACE=$ROOTLESS_WORKSPACE" \
	-e "QUARSHELL_E2E_SANDBOX_IMAGE=$E2E_SANDBOX_IMAGE" \
	"$NAME" "$E2E_INNER_TMP/podman.test" -test.run '^TestPodmanRootlessServiceRecovery$' -test.count=1 -test.v

