#!/usr/bin/env bash
set -euo pipefail

E2E_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_TMP="$(mktemp -d "${TMPDIR:-/tmp}/quarshell-e2e.XXXXXX")"
E2E_ARCH="$(go env GOARCH)"
E2E_HELPERS="$E2E_TMP/helpers"
E2E_FIXTURES="$E2E_TMP/fixtures"
E2E_SANDBOX_IMAGE="localhost/quarshell-connect-e2e:$(basename "$E2E_TMP")"
E2E_PROXY_IMAGE="localhost/quarshell-connect-e2e-proxy:$(basename "$E2E_TMP")"
E2E_SANDBOX_ARCHIVE="$E2E_TMP/sandbox-image.tar"
E2E_HOST_IMAGE_CREATED=0
mkdir -p "$E2E_HELPERS" "$E2E_FIXTURES/sandbox-image"

e2e_cleanup_tmp() {
	if [[ "${E2E_KEEP_TMP:-0}" == "1" ]]; then
		echo "Keeping E2E artifacts at $E2E_TMP"
	else
		rm -rf "$E2E_TMP"
	fi
}

e2e_build_artifacts() {
	echo "Building Linux runtime helpers and backend E2E fixtures ($E2E_ARCH)…"
	(cd "$E2E_ROOT/quarshell-cli" && GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go build -o "$E2E_HELPERS/quarshell" ./cmd/quarshell)
	(cd "$E2E_ROOT/quarshell-runtime" && GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go build -o "$E2E_HELPERS/quarshell-init" ./cmd/quarshell-init)
	(cd "$E2E_ROOT/quarshell-runtime" && GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go build -o "$E2E_HELPERS/quarshell-sshd" ./cmd/quarshell-sshd)
	(cd "$E2E_ROOT/quarshell-runtime" && GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go build -o "$E2E_HELPERS/quarshell-supervisor" ./cmd/quarshell-supervisor)
	GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go build -o "$E2E_FIXTURES/sandbox-image/quarshell-tcp-echo" "$E2E_ROOT/tools/e2e/tcp-echo/main.go"
	(cd "$E2E_ROOT/quarshell-driver" && GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go test -c -o "$E2E_TMP/docker.test" ./tests/e2e/docker)
	(cd "$E2E_ROOT/quarshell-driver" && GOOS=linux GOARCH="$E2E_ARCH" CGO_ENABLED=0 go test -c -o "$E2E_TMP/podman.test" ./tests/e2e/podman)
	cat >"$E2E_FIXTURES/sandbox-image/Containerfile" <<'EOF'
FROM docker.io/library/debian:bookworm-slim
COPY quarshell-tcp-echo /usr/local/bin/quarshell-tcp-echo
RUN chmod 0755 /usr/local/bin/quarshell-tcp-echo && printf 'sandbox:x:1000:1000:Sandbox:/sandbox:/bin/sh\n' >> /etc/passwd && printf 'sandbox:x:1000:\n' >> /etc/group && mkdir -p /sandbox /workspace /quarshell/data /openshell-volume && chown -R sandbox:sandbox /sandbox /workspace /quarshell/data /openshell-volume
WORKDIR /workspace
ARG QUARSHELL_E2E_USER=sandbox
USER ${QUARSHELL_E2E_USER}
EOF
}

e2e_build_sandbox_image() {
	echo "Building the shared sandbox fixture with the host Docker daemon…"
	docker build -t "$E2E_SANDBOX_IMAGE" -f "$E2E_FIXTURES/sandbox-image/Containerfile" "$E2E_FIXTURES/sandbox-image"
	E2E_HOST_IMAGE_CREATED=1
	docker build --build-arg QUARSHELL_E2E_USER=root -t "$E2E_PROXY_IMAGE" -f "$E2E_FIXTURES/sandbox-image/Containerfile" "$E2E_FIXTURES/sandbox-image"
	docker save -o "$E2E_SANDBOX_ARCHIVE" "$E2E_SANDBOX_IMAGE" "$E2E_PROXY_IMAGE"
}

e2e_remove_host_image() {
	if [[ "$E2E_HOST_IMAGE_CREATED" == "1" ]]; then
		docker image rm -f "$E2E_SANDBOX_IMAGE" "$E2E_PROXY_IMAGE" >/dev/null 2>&1 || true
	fi
}

e2e_wait_http() {
	local url="$1"
	local label="$2"
	for _ in $(seq 1 90); do
		if curl --fail --silent "$url" >/dev/null; then
			return 0
		fi
		sleep 1
	done
	echo "$label did not become ready at $url" >&2
	return 1
}

# Go's successful exit status also covers a fully skipped test. Required lanes
# must prove that the named test actually ran, otherwise a missing runtime
# binary or environment typo can silently turn compatibility coverage green.
e2e_run_required_test() {
	local label="$1"
	local test_name="$2"
	shift 2
	local log="$E2E_TMP/${label}.log"
	if ! "$@" >"$log" 2>&1; then
		cat "$log" >&2
		return 1
	fi
	cat "$log"
	if ! grep -F -- "--- PASS: $test_name" "$log" >/dev/null; then
		echo "$label did not run required test $test_name" >&2
		cat "$log" >&2
		return 1
	fi
}

# Some backend capabilities are intentionally unavailable on older disposable
# engines. Such skips are accepted only when the test emits the documented
# pinned limitation; every other skip remains a lane failure.
e2e_run_capability_test() {
	local label="$1"
	local test_name="$2"
	local expected_reason="$3"
	shift 3
	local log="$E2E_TMP/${label}.log"
	if ! "$@" >"$log" 2>&1; then
		cat "$log" >&2
		return 1
	fi
	cat "$log"
	if grep -F -- "--- PASS: $test_name" "$log" >/dev/null; then
		return 0
	fi
	if grep -F -- "--- SKIP: $test_name" "$log" >/dev/null && grep -F -- "$expected_reason" "$log" >/dev/null; then
		echo "$label skipped with documented capability limitation: $expected_reason" >&2
		return 0
	fi
	echo "$label did not pass required test $test_name or emit its documented capability limitation" >&2
	cat "$log" >&2
	return 1
}
