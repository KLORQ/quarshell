---
title: Remaining OpenShell migration work
---

# Remaining OpenShell migration work

Quarshell is still a pre-stable port of OpenShell. Matching RPC names and
message shapes does not establish behavioral parity, so the project does not
claim full compatibility yet.

The current acceptance plan tracks unfinished behavior across the public
control API, peer connections, credentials, provider lifecycle, runtime and
supervisor control, CLI and SDK behavior, package installation, and platform
backends. VM work still has partial acceptance rows. MXC and physical GPU/VFIO
support depend on platform hardware and SDK availability.

The [feature inventory](feature-inventory.yaml) records implementation and
evidence status per feature. The [detailed migration register](https://github.com/KLORQ/quarshell-workspace/blob/main/plans/REMAINING.md)
contains the active engineering checklist in Russian.

Until the remaining behavior and platform acceptance rows are closed, all Go
module versions remain pre-stable 0.x.
