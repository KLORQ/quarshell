# Policy schema differential evidence

- Verified: 2026-10-22
- Command: `task conformance:upstream`
- Candidate: `quarshell-policy` with the 71-case parse/validation/evaluation corpus
- Oracle: pinned OpenShell v0.1.4 (`5a7728c304b5528e954e9ae900cf21f84e1a12b1`)
- Host: Darwin arm64, Go 1.27.2, Rust 1.98.0
- Result: corpus 71/71 PASS; upstream differential 71/71 PASS
- Source digest: SHA-256 over sorted relative paths and file bytes for Go sources
  under `quarshell-policy/` and YAML corpus files under
  `plans/conformance/corpus/`.

The run built the pinned Rust oracle from the local baseline source and compared
the Go policy parser, validator, and evaluator outputs against it.
