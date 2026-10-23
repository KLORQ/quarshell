# quarshell-proto

This module contains the pinned OpenShell v0.1.4 protobuf contracts and
generated Go, gRPC, Connect, and TypeScript bindings. It has no first-party
`quarshell.control.v1` service.

## Pinned source and generation

The immutable upstream source copy and per-file SHA-256 lock are in
[`upstream/openshell/v0.1.4/proto`](upstream/openshell/v0.1.4/proto). Validate
and materialize the source, then generate the language bindings:

```sh
./scripts/check-openshell-proto-source.sh
python3 scripts/materialize-openshell-protos.py
buf lint
buf generate --template buf.gen.openshell.yaml --path proto/openshell
npm ci --prefix typescript
npm run generate --prefix typescript
npm run build --prefix typescript
```

The OpenShell template generates Go, gRPC, and Connect packages under
`gen/go/openshell`. The TypeScript package exports the public OpenShell
messages as `OpenShell*V1` namespaces; generated storage and test messages
remain internal to the repository.

CI and `scripts/pre-push.sh` verify the pinned source lock, lint the materialized
schema, and regenerate Go and TypeScript bindings to detect drift. The removed
Control API descriptor baseline is not used: the wire contract is OpenShell
v0.1.4.

## Toolchain

Requires Go 1.27.3, Buf 1.66.0, Node.js 22+, and npm. Generated Go and
TypeScript sources are checked in.
