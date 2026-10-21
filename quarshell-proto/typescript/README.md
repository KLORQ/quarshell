# OpenShell TypeScript client

Generated TypeScript bindings for the pinned OpenShell v0.1.4 contracts and a
Connect browser client for its Gateway service. No first-party control schema
is generated in this package.

```bash
npm ci
npm run check
```

Use `createControlTransport({ baseUrl: window.location.origin })` and
`createControlClients(transport)` from the same origin as the gateway. A
browser must use its own user session or OIDC credential; never place a
gateway owner or sandbox supervisor token in browser code.
