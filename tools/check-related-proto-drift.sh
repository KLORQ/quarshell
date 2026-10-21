#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UPSTREAM_VERSION="$(tr -d '[:space:]' < "$ROOT/tools/upstream/UPSTREAM_VERSION")"
PROTO="$ROOT/tools/upstream/openshell-$UPSTREAM_VERSION/proto"
GATEWAY="$ROOT/quarshell-gateway"
WRITE="${WRITE:-0}"
if [[ "$WRITE" == "1" ]]; then
  GEN="$GATEWAY/internal/upstreamproto"
else
  GEN="$(mktemp -d /tmp/cautem-related-proto.XXXXXX)"
fi
PROTOBUF_INCLUDE="/opt/homebrew/include"
if command -v brew >/dev/null 2>&1; then
  PROTOBUF_INCLUDE="$(brew --prefix protobuf)/include"
fi

common=(
  --go_opt=paths=source_relative
  --go-grpc_opt=paths=source_relative
  --go_opt=Moptions.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/optionsv1
  --go_opt=Mextension.proto=github.com/KLORQ/quarshell-gateway/internal/upstreamproto/extensionv1
  "--go_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/options.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/optionsv1"
  "--go_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/extension.proto=github.com/KLORQ/quarshell-gateway/internal/upstreamproto/extensionv1"
  --go_opt=Msandbox.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/sandboxv1
  "--go_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/sandbox.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/sandboxv1"
  --go_opt=Mdatamodel.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/datamodelv1
  "--go_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/datamodel.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/datamodelv1"
  --go_opt=Mopenshell.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/openshellv1
  "--go_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/openshell.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/openshellv1"
  --go-grpc_opt=Moptions.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/optionsv1
  --go-grpc_opt=Mextension.proto=github.com/KLORQ/quarshell-gateway/internal/upstreamproto/extensionv1
  "--go-grpc_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/options.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/optionsv1"
  "--go-grpc_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/extension.proto=github.com/KLORQ/quarshell-gateway/internal/upstreamproto/extensionv1"
  --go-grpc_opt=Msandbox.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/sandboxv1
  "--go-grpc_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/sandbox.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/sandboxv1"
  --go-grpc_opt=Mdatamodel.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/datamodelv1
  "--go-grpc_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/datamodel.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/datamodelv1"
  --go-grpc_opt=Mopenshell.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/openshellv1
  "--go-grpc_opt=Mtools/upstream/openshell-$UPSTREAM_VERSION/proto/openshell.proto=github.com/NVIDIA/OpenShell/sdk/go/proto/openshellv1"
)

generate() {
  local proto="$1" package="$2"
  mkdir -p "$GEN/$package"
  protoc -I "$PROTO" -I "$PROTOBUF_INCLUDE" \
    --go_out="$GEN/$package" --go-grpc_out="$GEN/$package" \
    "${common[@]}" \
    --go_opt="M$proto=github.com/KLORQ/quarshell-gateway/internal/upstreamproto/$package" \
    --go-grpc_opt="M$proto=github.com/KLORQ/quarshell-gateway/internal/upstreamproto/$package" \
    "$proto"
}

generate compute_driver.proto computev1
generate credential_driver.proto credentialsv1
generate supervisor_middleware.proto middlewarev1
generate gateway_interceptor.proto interceptorv1
generate extension.proto extensionv1

if [[ "$WRITE" == "1" ]]; then
  echo "related proto generated from OpenShell $UPSTREAM_VERSION source"
else
  for package in computev1 credentialsv1 middlewarev1 interceptorv1 extensionv1; do
    diff -ru "$GATEWAY/internal/upstreamproto/$package" "$GEN/$package"
  done
  echo "related proto drift: PASS (OpenShell $UPSTREAM_VERSION source + generated stubs)"
fi
