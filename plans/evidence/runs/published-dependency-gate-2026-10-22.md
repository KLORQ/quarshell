# Published dependency check

- Verified: 2026-10-22 (Europe/Samara)
- Command: `bash scripts/check-published-deps.sh`
- Host: macOS arm64, Go 1.27.2
- Result: FAIL; independent-consumer acceptance remains open.

The gate runs each Go module with `GOWORK=off`, `CGO_ENABLED=0`, and
`-mod=readonly`. Several modules fail because published dependency checksums
are absent from their `go.sum`; gateway/runtime also request `go.mod` updates
that readonly mode rejects. These failures are distinct from workspace builds,
which pass via the local file module proxy.

An explicit download check for `github.com/KLORQ/quarshell-core@v0.3.0` failed
Go's checksum verification:

- Downloaded module hash: `h1:sIbvCvsIGfAWENBarbrisQko7mdv1CqsyirTUfDtcZE=`
- Checksum database hash: `h1:mJy9zRdLUoK1mG390Wmd2FN/QUIV/6NeVu68xngO5xw=`

Do not bypass checksum verification. Reconcile the published source/tag with
the intended module contents, publish a new immutable version if needed, then
rerun the consumer gate. The failed check did not modify tracked module files.
