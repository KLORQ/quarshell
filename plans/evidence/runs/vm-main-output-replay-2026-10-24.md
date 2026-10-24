# Canonical VM process output replay and readers

This change ports a missing part of OpenShell `MainSession` output semantics into
the Linux VM guest agent.

Published as `quarshell-driver-vm v0.1.10`; `quarshell-gateway v0.1.10` now
depends on that VM version. Both standalone repository histories remain one
root commit.

## Behavior

- Canonical non-TTY stdout/stderr are retained in a bounded 1 MiB sequence log
  while a stream is attached or detached. A reconnect replays the retained tail,
  then receives live output and process exit.
- Multiple non-TTY attachments can read the same output sequence concurrently.
  Only one attachment at a time owns stdin. Other attachments are read-only;
  their stdin frames do not affect the process. A new attachment can claim stdin
  after the current owner disconnects. Closing stdin releases ownership and
  permanently closes process input.
- If a live reader falls behind the retained tail, the guest sends a `lagged`
  frame with the number of missing chunks. The VM stream converts it to an
  explicit output error rather than silently returning incomplete output.
- Canonical PTY output is retained as sequenced chunks while delivered live. Multiple attachments receive the same replay/live output; one attachment owns stdin; any attached client can resize the PTY or deliver an authorized process signal. Reconnect replays bytes that were already sent before transport loss.

## Verification

- Linux ARM64 full `vmguest` test binary passed in a privileged disposable
  Alpine container, including `TestRetainedMainProcessReplaysOutputFromPriorAttachment`,
  `TestRetainedMainProcessAllowsOutputReadersWithOneInputOwner`,
  `TestRetainedMainProcessReportsLaggedOutputCursor`, and
  `TestGuestRetainedTerminalCanReattachAfterTransportLoss`, and
  `TestGuestRetainedTerminalAllowsMultipleOutputReadersWithOneInputOwner`, plus
  `TestVMExecStreamsRejectInputWithoutOwnership` for host-side stdin ownership enforcement.
- Linux ARM64 full `vm` test binary passed in a privileged disposable Debian
  container.
- `GOWORK=off go test ./... -count=1` passed in the standalone Driver VM module
  on macOS ARM64.
- `task test` passed across all ten modules in the monorepo workspace after
  updating the Gateway dependency.
- `task vm:parity-check` passed for all 30 pinned OpenShell v0.1.4 entries.
- Latest `task vm:live-darwin` passed on Apple Silicon/HVF: VM lifecycle (28.79s), OCI import (1.16s), and Gateway watch composition (2.56s). The lifecycle acceptance covers main-process stream input, late TTY/pipe attachment, Gateway supervisor and policy flow, worker recovery, and shutdown.

## Disconnect retention follow-up

The same working tree now retains an interactive non-TTY exec process after its
control stream disconnects. `ExecStreamSessionID` exposes its stable ID;
`AttachExecStream` reattaches to that process; Gateway durable exec admission
records the ID and lets a duplicate request with the same payload resume it.
Changed payloads remain fenced, and runtimes without attach support still reject
duplicate requests.

- Linux ARM64 guest-agent regression passed in the disposable Alpine container:
  the test drops the initial stream, confirms the child stays alive, reattaches
  with the same ID, sends stdin, and receives stdout, stderr, and exit status.
- Gateway regression verifies duplicate non-TTY request attachment without a
  second runtime launch.
- `task vm:live-darwin` passed on Apple Silicon/HVF on 2026-10-24: VM lifecycle
  28.63s, OCI import 1.14s, and Gateway watch composition 2.63s. The live exec
  check detached and reattached to the retained guest process.
- `task test`, `task check`, `task vm:parity-check` (30 entries),
  `task fmt:check`, and `git diff --check` passed. The standalone Driver VM and
  Gateway modules also passed focused tests in a temporary workspace resolving
  their in-progress local Driver/VM APIs.

## Placement-neutral process handle follow-up

The Driver now exposes a `BoundaryProcess` handle from retained non-TTY exec
sessions. Its stable session ID supports attach, wait, allowlisted signals, and
detached process-group termination over the authenticated guest control
channel. Wait results remain stable across calls during the retained-session
window. Reconnect retries the attach handshake until stdin ownership has been
released by the prior stream, avoiding a read-only retry attachment during the
brief disconnect handoff race.

- Linux ARM64 Alpine guest tests passed five repetitions for stable wait,
  detached signal, detached process-group terminate, and reconnect after
  transport loss.
- Apple Silicon/HVF `task vm:live-darwin` passed on 2026-10-24: VM lifecycle
  29.09s, OCI import 1.13s, and Gateway watch composition 2.65s. Live acceptance
  checked process-handle exposure, repeated wait after output completion, and
  detached termination.
- `task test`, `task check`, `task vm:parity-check` (30 entries),
  `task fmt:check`, and `git diff --check` passed for this change set.

## Remaining scope

The parity row remains partial for durable per-reader cursor lineage across
Gateway replica handoff. The subsequent section records the aggregate
`RunningBoundary` implementation.

## Canonical workload exit forwarding

Gateway forwards the first durable, generation-bound supervisor exit result
through the runtime adapter to backends implementing
`MainProcessExitRecorder`. Duplicate reports forward the persisted first
instance and code, so a later conflicting report cannot replace the backend's
terminal result. The VM driver accepts this report after its worker has
stopped, preserving the stopped lifecycle state while making the exit code
available to `BoundaryProcess.Wait`.

`BoundaryProcess.Terminate` for the canonical workload sends KILL and returns
independently; `Wait` receives the durable supervisor result. Guest shutdown
can precede the authenticated report reaching Gateway and the VM backend.

Validation on 2026-10-24: focused Gateway and VM tests, full
`task vm:live-darwin` (HVF lifecycle, canonical KILL exit 137, Gateway
supervisor flow, OCI import and Gateway watch), standalone module tests,
`task vm:parity-check`, `task test`, `task check`, `task fmt:check`, and
`git diff --check` passed at this checkpoint. The parity row remained partial
for `RunningBoundary` and replica cursor lineage until the follow-up below.

## Running boundary aggregate

The driver now exposes one `RunningBoundary` with the canonical agent process,
in-boundary TTY and non-TTY exec, VM-tree termination, and a loopback connector.
The VM connector opens an authenticated guest control stream; the guest accepts
only literal loopback IPs and valid TCP ports, then bridges framed data to a
TCP socket inside the VM. This avoids exposing a host-side guest PID or dialing
the target from the host network namespace.

Apple Silicon/HVF acceptance on 2026-10-24 starts a TCP echo helper inside the
guest, connects through the aggregate connector, verifies the echoed payload,
runs both non-TTY and PTY exec through the same handle, resizes the PTY, waits
for process exit, and terminates the owning VM boundary. Linux guest tests
cover loopback stream bridging and reject non-loopback addresses. Gateway now
stores non-TTY exec output cursors with durable request admission and resumes
guest output after the last acknowledged sequence on same-request reconnect
through another Gateway. TTY exec attaches use the same request cursor contract.
Canonical main-process streams accept a stable `reader_id` and persist its
cursor in the shared lifecycle store for both PTY and separate stdout/stderr
attachments. Cursor updates are monotonic and happen after the output event is
sent, so a disconnect at the send/checkpoint boundary may replay the last
frame; delivery is at-least-once across that boundary.

The complete `task vm:live-darwin` gate passed on this revision: HVF lifecycle
29.07s, local OCI import 1.23s, and Gateway watch composition 2.55s. Focused
standalone Driver and VM module tests pass. Full workspace `task test`,
`task check`, formatting, and diff checks are recorded above; the final aggregate
contract follow-up also passed full workspace `task check` (including lint,
`go vet`, `govulncheck`, and all module tests), `task vm:parity-check`,
`task fmt:check`, and `git diff --check`. Standalone Driver and VM focused tests
pass on the synchronized module copies. Cursor recovery is covered by the Linux
guest test that resumes after sequence 1 without replaying it, and the Gateway
SQLite reopen tests that verify exec and canonical reader cursors survive and
cannot move backwards. VM tests also cover PTY frame cursor propagation. The
full live HVF suite passed again (VM lifecycle 30.07s, OCI import 1.14s,
Gateway watch 2.62s). Full workspace `task test`, `task check`, 30-row parity,
formatting/diff gates, Linux ARM64 guest suite, and focused standalone Driver,
VM, Gateway, Proto, and SDK tests passed.
