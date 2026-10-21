# Gateway image local acceptance

- Verified: 2026-10-22 (Europe/Samara)
- Host: macOS Docker Desktop, built target `linux/arm64`
- Image: `quarshell-gateway:migration-check`
- Image ID: `sha256:c46a2a9f53b569cd46322d7c5912f9457cae7b774513a81bf4bcff5464c42c35`
- Result: build and startup healthcheck PASS

The image was built from the repository root with:

```sh
docker build --file quarshell-gateway/Dockerfile --tag quarshell-gateway:migration-check .
```

The disposable container started with a synthetic local-only `QUARSHELL_TOKEN`
and the image's default command. HTTP returned `{"status":"ok"}` without
credentials, gRPC listened on `0.0.0.0:7444`, and Docker reported the configured
image healthcheck as `healthy`. The container was then stopped and removed. The
Linux workflow repeats the image build, HTTP healthcheck, and gRPC port probe on
`ubuntu-24.04`; registry publication remains pending.
