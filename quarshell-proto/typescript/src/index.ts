// OpenShell v0.1.4 public contracts generated from the same pinned schemas as Go.
// Namespaces keep upstream package boundaries visible and prevent collisions
// between similarly named messages in separate protobuf packages.
export * as OpenShellComputeV1 from "./gen/openshell/compute/v1/compute_driver_pb.js";
export * as OpenShellCredentialsV1 from "./gen/openshell/credentials/v1/credential_driver_pb.js";
export * as OpenShellDatamodelV1 from "./gen/openshell/datamodel/v1/datamodel_pb.js";
export * as OpenShellExtensionV1 from "./gen/openshell/extension/v1/extension_pb.js";
export * as OpenShellGatewayV1 from "./gen/openshell/gateway/v1/openshell_pb.js";
export * as OpenShellGatewayInterceptorV1 from "./gen/openshell/gateway_interceptor/v1/gateway_interceptor_pb.js";
export * as OpenShellMiddlewareV1 from "./gen/openshell/middleware/v1/supervisor_middleware_pb.js";
export * as OpenShellOptionsV1 from "./gen/openshell/options/v1/options_pb.js";
export * as OpenShellPaginationV1 from "./gen/openshell/pagination/v1/pagination_pb.js";
export * as OpenShellSandboxV1 from "./gen/openshell/sandbox/v1/sandbox_pb.js";
export * as OpenShellSandboxProtocolV1 from "./gen/openshell/sandbox_protocol/v1/openshell_sandbox_pb.js";

import { createClient, type Interceptor, type Transport } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { OpenShell } from "./gen/openshell/gateway/v1/openshell_pb.js";

export interface ControlClientOptions {
  baseUrl: string;
  fetch?: typeof globalThis.fetch;
  interceptors?: Interceptor[];
}

export function createControlTransport(options: ControlClientOptions): Transport {
  return createConnectTransport({
    baseUrl: options.baseUrl,
    fetch: options.fetch,
    interceptors: options.interceptors,
    useBinaryFormat: true,
  });
}

export function createControlClients(transport: Transport) {
  return {
    gateway: createClient(OpenShell, transport),
  };
}
