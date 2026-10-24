import assert from "node:assert/strict";

import { createControlClients, createControlTransport } from "../dist/index.js";

const baseUrl = process.env.QUARSHELL_GATEWAY_URL;
const token = process.env.QUARSHELL_GATEWAY_TOKEN;
assert.ok(baseUrl, "QUARSHELL_GATEWAY_URL is required");
assert.ok(token, "QUARSHELL_GATEWAY_TOKEN is required");

const transport = createControlTransport({
  baseUrl,
  interceptors: [
    (next) => async (request) => {
      request.header.set("Authorization", `Bearer ${token}`);
      return next(request);
    },
  ],
});
const { gateway } = createControlClients(transport);

assert.equal((await gateway.health({})).status, 1);
const info = await gateway.getGatewayInfo({});
assert.equal(info.gatewayVersion, "0.0.0-dev");

const name = `typescript-${Date.now()}`;
const created = await gateway.createWorkspace({
  name,
  labels: { source: "typescript-connect-e2e" },
});
assert.equal(created.workspace.metadata.name, name);

const found = await gateway.getWorkspace({ name });
assert.equal(found.workspace.metadata.labels.source, "typescript-connect-e2e");

const listed = await gateway.listWorkspaces({});
assert.ok(listed.workspaces.some((workspace) => workspace.metadata.name === name));

await gateway.deleteWorkspace({ name });
console.log("TypeScript Connect client Gateway acceptance passed");
