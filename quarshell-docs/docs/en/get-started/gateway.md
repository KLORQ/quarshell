<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Start the gateway

The Go Gateway requires a private `QUARSHELL_TOKEN`. HTTP requests and gRPC
calls use this token as a Bearer credential, except for HTTP liveness and gRPC
`Health`. Keep the token in your shell environment or a local ignored `.env`
file. The shared token represents the static `gateway-client` administrator;
it is not a per-user or sandbox identity.

### Docker Compose

From the repository root, generate a local token and start the Gateway:

```sh
export QUARSHELL_TOKEN="$(openssl rand -hex 32)"
docker compose -f quarshell-gateway/compose/docker-compose.yml up --build -d
curl --fail https://127.0.0.1:17670/v1/healthz
```

Compose stores state in the `quarshell-gateway-data` volume. HTTP is available
on `127.0.0.1:17670`; gRPC is available on `127.0.0.1:7445`.
gRPC has no TLS listener. Keep port `7445` on loopback or a trusted private
network; do not publish it to untrusted clients.

### Local binary

From the repository root:

```sh
export QUARSHELL_TOKEN="$(openssl rand -hex 32)"
go -C quarshell-gateway run ./cmd/quarshell-gateway \
  --listen 127.0.0.1:17670 \
  --grpc-listen 127.0.0.1:7444 \
  --runtime docker
```

Without `--data`, the Gateway keeps state in memory. Set `--data ./state.db`
to use SQLite. The `/v1/healthz` endpoint returns only `{"status":"ok"}`.

The UI and Dex compose profiles are retained from the migration baseline but
are not integrated with the current token-authenticated Gateway. See the
[Gateway module README](https://github.com/KLORQ/quarshell/blob/master/quarshell-gateway/README.md)
and [remaining migration work](https://github.com/KLORQ/sandbox.dev/blob/main/docs/ru/reference/remaining-migration-work.md)
for current scope.
