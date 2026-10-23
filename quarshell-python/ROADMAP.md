# Roadmap — quarshell-python

Status: pre-stable SDK · Package publication and installed-consumer acceptance are tracked below.

## This module

| ID | Item | Notes |
|----|------|-------|
| Y1 | **API parity** | Provider CRUD/secret patch/attachment/refresh, provider profiles, workspace/member administration, SSH session issue/revoke, sandbox service endpoints, rootfs staging upload, typed gateway settings, audit APIs, sandbox inventory/log streaming, global and sandbox policy/revisions, proposal review, and interactive exec streams are implemented against the pinned contract. Interactive SSH terminal connect remains CLI-only. Published-consumer behavior and remaining release-pinned leaf coverage remain open. |
| Y2 | **Typing** | `py.typed` and generated protocol stubs ship in the package. The public client surface is marked partially typed; complete annotations/stubs remain open. |
| Y3 | **PyPI** | Versioned `quarshell` package aligned with gateway alpha; publication and installed-consumer acceptance remain open. |


## Release

This module follows its own pre-stable 0.x version sequence. Publish dependencies
first, update consumers to the published tags, and verify with `GOWORK=off`. Never
rewrite a published tag.
