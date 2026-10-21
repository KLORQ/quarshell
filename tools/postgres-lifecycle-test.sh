#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAME="quarshell-postgres-lifecycle"
PORT="${QUARSHELL_POSTGRES_PORT:-55433}"

cleanup() {
	docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
docker run --rm -d --name "$NAME" \
	-e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=test \
	-p "127.0.0.1:${PORT}:5432" postgres:16-alpine >/dev/null

for _ in $(seq 1 60); do
	if docker exec "$NAME" pg_isready -U test >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
if ! docker exec "$NAME" pg_isready -U test >/dev/null 2>&1; then
	echo "postgres did not become ready" >&2
	exit 1
fi

QUARSHELL_POSTGRES_URL="postgres://test:test@127.0.0.1:${PORT}/test?sslmode=disable" \
	go -C "$ROOT/quarshell-gateway" test ./internal/application/lifecycle -run TestPostgresStore -count=1
