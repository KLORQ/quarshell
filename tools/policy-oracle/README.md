# Policy oracle

Small Rust binary that calls the parser from the pinned upstream checkout at
`tools/upstream/openshell-v0.1.4/crates/openshell-policy`. It is used
only by the differential conformance runner, never by the Go product.

```bash
cargo build --manifest-path tools/policy-oracle/Cargo.toml --release
tools/policy-oracle/target/release/policy-oracle <policy.yaml>
tools/policy-oracle/target/release/policy-oracle --validate <policy.yaml>
```

`--validate` also runs the full runtime `validate_sandbox_policy` pass.

The differential gate runs both the parse and validation corpora:

```bash
task conformance:upstream
```

The upstream checkout must exist and `cargo` must be available. If either is
missing the gate fails as `blocked`; it does not fall back to local
expectations.
