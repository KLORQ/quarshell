# Contributing to slogx

Thank you for your interest in contributing!

## Requirements

- **Go 1.27+**
- **golangci-lint** for code quality checks

## Quick Start

```bash
git clone https://github.com/KLORQ/quarshell-slogx
cd slogx

go build ./...
go test -race ./...
golangci-lint run --timeout=5m
```

## Development Workflow

1. Fork and clone the repository
2. Create a feature branch (`feat/...`, `fix/...`)
3. Make changes with tests
4. Run the pre-release checks:

```bash
make pre-release
# or quick (skip coverage + lint):
make pre-release-quick
```

5. Open a pull request against `main`

## Tests

```bash
make test
make coverage
make examples
```

Examples under `examples/` are build-checked in CI and excluded from coverage.

## Commit style

Prefer conventional commits:

- `feat:` new capability
- `fix:` bug fix
- `test:` tests only
- `docs:` documentation
- `chore:` tooling / CI

Author: `lkmavi <zikmanv@icloud.com>` unless specified otherwise.
