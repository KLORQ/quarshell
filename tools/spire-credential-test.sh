#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIR="$(mktemp -d "${TMPDIR:-/tmp}/quarshell-spire.XXXXXX")"
NETWORK="quarshell-spire-net"
SERVER="quarshell-spire-server"
AGENT="quarshell-spire-agent"
KEEP="${QUARSHELL_SPIRE_KEEP:-0}"

cleanup() {
	docker rm -f "$AGENT" "$SERVER" >/dev/null 2>&1 || true
	docker network rm "$NETWORK" >/dev/null 2>&1 || true
	if [[ "$KEEP" != "1" ]]; then
		rm -rf "$DIR"
	else
		echo "kept SPIRE state: $DIR"
	fi
}
trap cleanup EXIT

mkdir -p "$DIR/server" "$DIR/server-data" "$DIR/agent" "$DIR/agent-data" "$DIR/agent-socket"
chmod 777 "$DIR/agent-socket"

cat > "$DIR/server/server.conf" <<'CONF'
server {
  bind_address = "0.0.0.0"
  bind_port = "8081"
  trust_domain = "example.org"
  data_dir = "/run/spire/data"
  log_level = "DEBUG"
}
plugins {
  DataStore "sql" {
    plugin_data {
      database_type = "sqlite3"
      connection_string = "/run/spire/data/datastore.sqlite3"
    }
  }
  NodeAttestor "join_token" { plugin_data {} }
  KeyManager "disk" { plugin_data { keys_path = "/run/spire/data/keys.json" } }
}
health_checks {
  listener_enabled = true
  bind_address = "0.0.0.0"
  bind_port = "8080"
  live_path = "/live"
  ready_path = "/ready"
}
CONF

cat > "$DIR/agent/agent.conf" <<'CONF'
agent {
  data_dir = "/run/spire/data"
  log_level = "DEBUG"
  server_address = "quarshell-spire-server"
  server_port = "8081"
  trust_domain = "example.org"
  socket_path = "/run/spire/sockets/agent.sock"
  insecure_bootstrap = true
}
plugins {
  NodeAttestor "join_token" { plugin_data {} }
  KeyManager "disk" { plugin_data { directory = "/run/spire/data" } }
  WorkloadAttestor "unix" { plugin_data {} }
}
health_checks {
  listener_enabled = true
  bind_address = "0.0.0.0"
  bind_port = "8080"
  live_path = "/live"
  ready_path = "/ready"
}
CONF

docker network create "$NETWORK" >/dev/null
docker run -d --name "$SERVER" --network "$NETWORK" \
	-v "$DIR/server:/opt/spire/conf/server:ro" \
	-v "$DIR/server-data:/run/spire/data" \
	ghcr.io/spiffe/spire-server:1.9.5 -config /opt/spire/conf/server/server.conf >/dev/null

for _ in $(seq 1 60); do
	if docker exec "$SERVER" /opt/spire/bin/spire-server healthcheck >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
if ! docker exec "$SERVER" /opt/spire/bin/spire-server healthcheck >/dev/null 2>&1; then
	echo "spire-server did not become ready" >&2
	docker logs "$SERVER" 2>&1 | tail -20
	exit 1
fi

TOKEN="$(docker exec "$SERVER" /opt/spire/bin/spire-server token generate -spiffeID spiffe://example.org/agent | awk '/Token:/ {print $2; exit}')"
if [[ -z "$TOKEN" ]]; then
	echo "could not generate join token" >&2
	exit 1
fi

docker run -d --name "$AGENT" --network "$NETWORK" --pid=host \
	-v "$DIR/agent:/opt/spire/conf/agent:ro" \
	-v "$DIR/agent-data:/run/spire/data" \
	-v "$DIR/agent-socket:/run/spire/sockets" \
	ghcr.io/spiffe/spire-agent:1.9.5 -config /opt/spire/conf/agent/agent.conf -joinToken "$TOKEN" >/dev/null

for _ in $(seq 1 60); do
	if [[ -S "$DIR/agent-socket/agent.sock" ]]; then
		break
	fi
	sleep 1
done
if [[ ! -S "$DIR/agent-socket/agent.sock" ]]; then
	echo "spire-agent did not create its workload socket" >&2
	docker logs "$AGENT" 2>&1 | tail -20
	exit 1
fi
chmod 777 "$DIR/agent-socket/agent.sock" || true

docker exec "$SERVER" /opt/spire/bin/spire-server entry create \
	-parentID spiffe://example.org/agent \
	-spiffeID spiffe://example.org/workload \
	-selector "unix:uid:0" >/dev/null

ARCH="$(uname -m)"
case "$ARCH" in
	arm64|aarch64) GOARCH=arm64 ;;
	x86_64|amd64) GOARCH=amd64 ;;
	*) echo "unsupported host architecture: $ARCH" >&2; exit 1 ;;
esac
mkdir -p "$DIR/test"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go -C "$ROOT/quarshell-gateway" test -c -o "$DIR/test/spiffe.test" ./credentialstore

sleep 2
if ! docker run --rm --network "$NETWORK" \
	-v "$DIR/agent-socket:/run/spire/sockets" \
	-v "$DIR/test:/test:ro" \
	-e QUARSHELL_SPIFFE_SOCKET=/run/spire/sockets/agent.sock \
	alpine:3.20 /test/spiffe.test -test.run TestSpiffeJWTSourceIntegrationLive -test.v; then
	echo "--- agent log ---" >&2
	docker logs "$AGENT" 2>&1 | tail -40 >&2
	exit 1
fi
