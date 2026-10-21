# Roadmap — quarshell-python

Status: **v0.1.7** (release prepared) · Python SDK for [quarshell-gateway](https://github.com/KLORQ/quarshell-gateway)

## This module

| ID | Item | Notes |
|----|------|-------|
| Y1 | **API parity** | Provider CRUD/secret patch/attachment/refresh, provider profiles, workspace/member administration, SSH session issue/revoke, sandbox service endpoints, rootfs staging upload, typed gateway settings, audit APIs, sandbox inventory/log streaming, global and sandbox policy/revisions, proposal review, and interactive exec streams are implemented against the pinned contract. Interactive SSH terminal connect remains CLI-only. Published-consumer behavior and remaining release-pinned leaf coverage remain open. |
| Y2 | **Typing** | `py.typed` and generated protocol stubs ship in the package. The public client surface is marked partially typed; complete annotations/stubs remain open. |
| Y3 | **PyPI** | Versioned `quarshell` package aligned with gateway alpha; publication and installed-consumer acceptance remain open. |

## Release

Independent package version · generated first-party clients come from `quarshell-proto`.
