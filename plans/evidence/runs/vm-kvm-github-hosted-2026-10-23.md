# Linux KVM acceptance on a GitHub-hosted runner

- Workflow: [quarshell-driver / Live VM KVM acceptance, run 40027323789](https://github.com/KLORQ/quarshell-driver/actions/runs/40027323789)
- Driver snapshot: `09fcd12deb13e986bfb9b73056b0da6ad5c88380` (single root commit)
- Runner: `ubuntu-24.04`, GitHub-hosted; `/dev/kvm` opened successfully.
- Runtime: libkrun and libkrunfw built from pinned OpenShell v0.1.4 source commit `5a7728c304b5528e954e9ae900cf21f84e1a12b1`; generated runtime trust lock was staged in the job.
- Workspace modules: pinned Core, Driver VM, Runtime, Proto, Proxy, and CLI checkouts; CLI v0.1.6 was built for paired Gateway credential acceptance.

## Result

The entire workflow completed successfully. Its live acceptance command passed:

- `TestLiveMicroVMLifecycle`: VM create/start/stop/recovery, policy and filesystem checks, supervisor/Gateway credentials, canonical PTY, natural workload exit and worker shutdown, signal exit 137, retained TTY and non-TTY late attachment, archive transfer, and Gateway watch composition.
- `TestLiveMicroVMNestedDocker`.
- `TestLiveMicroVMNestedPodman` with Podman 6.1.4.
- `TestLiveMicroVMNestedPodmanRootless` with Podman 6.1.4.

The run exposed an acceptance fixture defect before it passed: the Podman Containerfile already creates UID 1000 user `sandbox`, while the wrapper script attempted to create it a second time. The wrapper now asserts `id sandbox` exists and proceeds to export that rootfs. Both rootful and rootless Podman scenarios passed after the fix.

The KVM job runs in a disposable privileged Docker container on a GitHub-hosted runner. The workflow checks out and compiles its pinned runtime inputs per run; the build step took about 21 minutes on this runner, followed by roughly five minutes for all acceptance scenarios.

## Remaining OpenShell parity

This successful hardware acceptance closes the live Linux/KVM verification gap. It does not by itself close the three `partial` host supervisor/session entries or the two GPU/VFIO entries in `vm/testdata/openshell-v0.1.4-parity.json`. GPU inventory/allocation and crash recovery still require hardware-backed acceptance.
