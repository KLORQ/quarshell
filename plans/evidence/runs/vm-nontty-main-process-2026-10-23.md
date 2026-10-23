# VM non-TTY canonical main-process attachment

Date: 2026-10-23, Apple Silicon macOS/HVF

## Implemented path

- VM start now assigns a durable canonical process session ID for both TTY and
  non-TTY workloads, while retaining the previous state field for existing
  state files.
- The Linux guest agent retains non-TTY stdin/stdout/stderr pipes independently
  of a stream connection, allows one input owner at a time, replays a bounded
  output tail after reconnect, and reports the normalized exit code. Output
  readers drain for up to two seconds after the process exits before closing
  inherited descriptors.
- A Quarshell runtime RPC and Go SDK path carries the separate streams. The
  additive `non_tty` field defaults to false, preserving TTY behavior for
  clients generated from the older extension schema.

## Verification

- Linux ARM64 guest-agent suite passed inside `alpine:3.22`, including
  `TestRetainedMainProcessStreamsPipesAndAcceptsInput` and
  `TestStartWorkloadRetainsNonTTYCanonicalProcessOutput`.
- Gateway RPC acceptance verified stdin, separate stdout/stderr, and exit code.
- SDK tests verified both the default TTY attachment and non-TTY selection.
- Local Apple Silicon/HVF `task vm:live-darwin` passed the full VM lifecycle and
  OCI import. The lifecycle includes a late non-TTY attachment to a short-lived
  workload and verifies retained stdout, stderr, and exit code 9.

## Remaining upstream parity

The live non-TTY acceptance above uses the VM guest agent's direct workload
path. With the pinned OpenShell supervisor enabled, the canonical process is
owned by OpenShell's `MainSession` and attached through the SSH
`openshell-main` subsystem. Quarshell does not yet route the extension to that
supervisor-owned session, so this implementation does not close the upstream
supervisor process-handle or replica/session replay rows. Those remain partial
in `plans/REMAINING.md` and the 30-row parity inventory.
