# VM `openshell-main` SSH subsystem acceptance (2026-10-23)

## Implemented path

The VM guest agent now exposes a root-only Unix control socket for the in-guest
SSH daemon. The daemon reads the current root-only guest-control token for each
attachment and accepts only the `openshell-main` SSH subsystem for canonical
process attachment. Guest token rotation updates the private file atomically.

The subsystem bridges SSH channel stdin/EOF, stdout, stderr and exit status to
the retained canonical guest process. PTY requests forward initial dimensions
and later resize events; only the existing TERM, KILL, INT and HUP signal
allowlist is forwarded. TTY attachment reuses the retained PTY session; pipe
based processes retain separate output streams. Detached PTY replay is bounded
to the pinned upstream 1 MiB output limit.

## Verification

- `task vm:live-darwin`: PASS on Apple Silicon/HVF. `TestLiveMicroVMLifecycle`
  (29.34 s) opened `openshell-main` through the VM SSH relay and checked input
  echo plus a 24x80 PTY; the same lifecycle test also covered worker adoption,
  natural exit, KILL, delayed TTY/non-TTY attachment and Gateway supervisor
  flow. Local OCI import passed (1.13 s); Gateway live watch composition
  passed (2.44 s).
- Linux ARM64 test binaries executed in `alpine:3.22`: VM guest-agent suite
  passed; runtime SSH subsystem integration passed separate stdout/stderr,
  stdin half-close and exit status 7.
- Full `go test -C quarshell-driver-vm ./...`,
  `go test -C quarshell-runtime ./...`,
  `go test -C quarshell-core ./defaults/...`, `task vm:parity-check`
  (30 pinned rows), and `git diff --check` passed.

## Remaining upstream differences

This is a Quarshell SSH implementation backed by the retained guest process. It
does not yet reproduce upstream Rust `MainSession` replica behavior, output
cursor lag reporting, or host-side `BoundaryProcess`/`RunningBoundary`
ownership. Those gaps remain partial in `vm.host-supervisor-isolation` and
`vm.host-supervisor-bootstrap`.

The latest hosted KVM run (`39946541169`) predates this change. Its live test
timed out after reaching a natural workload exit and waiting for the VM worker
to stop; KVM lifecycle is still unverified.
