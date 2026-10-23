# VM host-initiated vsock acceptance (2026-10-23)

## Environment

- Host: Apple Silicon macOS, Hypervisor.framework (HVF).
- Runtime: OpenShell-pinned libkrun bundle prepared by the acceptance script.
- Source revision under test: transport changes later committed as `47a15b2` (Windows build stub follows in `f8eef13`).

## Command and result

```text
task vm:live-darwin
```

The command passed all required live tests:

- `TestLiveMicroVMLifecycle` — PASS (28.45 s).
- `TestLiveLocalOCIEngineImport` — PASS (1.30 s).
- `TestLiveVMWatchSandboxComposition` — PASS (14.70 s).

## Coverage observed

- Guest policy proxy egress passed deny, allow, and revoke probes through the host-initiated vsock bridge on port 5501.
- The guest supervisor established its mTLS control stream through port 5503; Gateway accepted the Sandbox bearer, acknowledged policy revision 2, and received a supervisor process event.
- HVF lifecycle covered recovery/adoption, explicit restart, worker/broker loss, OCI import and immutable-image recovery, workload exit, retained TTY, and shutdown.
- Gateway `WatchSandbox` composition passed against the live VM.

This proves the transport on local HVF. The separate hosted KVM lifecycle run is tracked in `plans/REMAINING.md` and remains the Linux/KVM acceptance gate.

## KVM mTLS bridge follow-up

Hosted run `39931018714` exercised the same flow on KVM and exposed an idle-stream ordering bug: the supervisor forwarder opened the host proxy endpoint before receiving any guest bytes. The proxy TLS server then waited for a ClientHello on a connection that had no guest client, while the actual KVM stream was left without a bridge. Commit `09985e1` now waits for the guest's initial bytes before dialing the host proxy and runs a pool of eight bridge workers.

After that change, the following tests passed locally on HVF:

- `TestLiveMicroVMLifecycle` — PASS (36.22 s), including supervisor mTLS reconnect, Gateway credential/policy flow, worker recovery, stop/start, broker-loss fencing, OCI pull/recovery, retained TTY, and shutdown.
- `TestLiveLocalOCIEngineImport` — PASS (1.16 s).
- `TestLiveVMWatchSandboxComposition` — PASS (2.78 s), including live Gateway create/start/watch/stop over HVF.

The Gateway acceptance fixture now serves the staged rootfs from an in-process OCI registry, avoiding a Docker Hub dependency. A new hosted KVM result is still required to confirm the forwarder fix on Linux.

## Full task follow-up

The complete `task vm:live-darwin` lane passed again after the mTLS forwarder change, using the cached, hash-verified umoci artifact and cached Docker image:

- `TestLiveMicroVMLifecycle` — PASS (28.54 s).
- `TestLiveLocalOCIEngineImport` — PASS (1.16 s).
- `TestLiveVMWatchSandboxComposition` — PASS (2.63 s).

The runner script now skips `docker pull` when its configured local image already exists. The Gateway composition test serves the staged rootfs from a local OCI registry, so these live VM checks completed without Docker Hub access. Hosted KVM remains the cross-hypervisor confirmation gate.
