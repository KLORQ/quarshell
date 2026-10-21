# slogx

The local workspace checkout is named `quarshell-slogx`. The published repository and Go module path remain `github.com/KLORQ/quarshell-slogx` until a new release path is available.
See [cautem development and releases](https://klorq.github.io/sandbox.dev/reference/development/) for the shared workspace and release process.

[![CI](https://github.com/KLORQ/quarshell-slogx/actions/workflows/ci.yml/badge.svg)](https://github.com/KLORQ/quarshell-slogx/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/KLORQ/quarshell-slogx.svg)](https://pkg.go.dev/github.com/KLORQ/quarshell-slogx)
[![Go 1.27+](https://img.shields.io/badge/Go-1.27+-blue)](https://go.dev/dl/)

Structured logging for Go, built on [`log/slog`](https://pkg.go.dev/log/slog).
It adds trace correlation, redaction, live level control, and bounded object dumping.

## Features

- **Runtime configuration** — change level, format (JSON/Text), masking, and removal without restart
- **Live level control** — `SetLevel` / env watch (`LOG_LEVEL`) / HTTP `GET|PUT /level`
- **Corporate masking** — passwords, tokens, JWT, AWS keys, PEM, PAN (Luhn), IBAN, email, phone; auto-detect + fingerprints
- **OpenTelemetry** — automatic `trace_id`, `span_id`, and `trace_sampled` from `context.Context`
- **Stack traces** — optional stack attachment on `Error` and `Fatal` records
- **Typed context keys** — collision-safe `ContextKey`; logger in context via `ToContext` / `FromContext`
- **Fast dumping** — struct serialization via saferefl (depth limits, cycles, field caps)
- **Payload helpers** — sanitized HTTP and gRPC request/response snapshots

## Install

The current published alpha tag still declares an older module path, so `go get`
from outside this workspace is not yet supported. For local development, use
the sibling `go.work` workspace and run `go test ./...` in this checkout.
The import path below is the intended path for the next coordinated release.

## Quick start

```go
package main

import (
    "context"
    "log/slog"

    "github.com/KLORQ/quarshell-slogx"
)

func main() {
    log := slogx.New(
        slogx.WithFormat(slogx.FormatJSON),
        slogx.WithTraceContext(true),
        slogx.WithStackOnError(true),
        slogx.WithContextKeys(slogx.KeyRequestID),
        slogx.WithMaskRules(
            slogx.NewMaskRules().Add("email", slogx.MaskEmail),
        ),
    )
    slog.SetDefault(log.Logger)

    ctx := slogx.WithValue(context.Background(), slogx.KeyRequestID, "req-1")
    log.InfoContext(ctx, "started", slog.String("email", "user@example.com"))
}
```

## Presets

```go
log := slogx.Default()    // JSON, Info, OTel, stack, corporate masking
log := slogx.Dev()        // Text, Debug, OTel, stack, corporate masking
log := slogx.Corporate()  // alias of Default()
log := slogx.New(slogx.WithCorporateMasking())
```

## Live log level

```go
log.SetLevelString("debug")
go log.WatchLevelEnv(ctx, "LOG_LEVEL", 0) // poll env
```

## Examples

Runnable examples live under [`examples/`](examples/):

| Example | What it shows |
| --- | --- |
| [`examples/basic/`](examples/basic/) | levels, formats, `Err`, `NewNop` |
| [`examples/masking/`](examples/masking/) | email/phone/card/secret masking + removal |
| [`examples/context/`](examples/context/) | typed context keys, `ToContext` / `FromContext` |
| [`examples/tracing/`](examples/tracing/) | OpenTelemetry `trace_id` / `span_id` injection |
| [`examples/stack/`](examples/stack/) | `AddSource`, stack on Error/Fatal, `WithExitFunc` |
| [`examples/dump/`](examples/dump/) | `Dump` / `DumpGroup` via saferefl |
| [`examples/http_payload/`](examples/http_payload/) | sanitized HTTP request/response dumps |
| [`examples/grpc_payload/`](examples/grpc_payload/) | sanitized gRPC call/response/stream dumps |
| [`examples/runtime_config/`](examples/runtime_config/) | `UpdateConfig` / `SetLevel` hot reload |
| [`examples/level_control/`](examples/level_control/) | Runtime log-level updates |
| [`examples/presets/`](examples/presets/) | `Default`, `Dev`, `MustDefault` |

```bash
go run ./examples/basic/
go run ./examples/tracing/
go run ./examples/dump/
make examples
```

## Dumping

```go
log.Info("payload",
    slogx.DumpWith(log, "user", user),
)
```

## HTTP / gRPC payloads

```go
log.Info("http", slogx.HTTPRequestWith(log, "request", req))
log.Info("grpc", slogx.GRPCCallWith(log, "call", "/svc.Method", body, md))
```

Sensitive headers (`Authorization`, `Cookie`, …) are redacted automatically.
`HTTPRequestWith` restores `http.Request.Body` for downstream handlers.

## Runtime config

```go
log.UpdateConfig(func(c *slogx.Config) {
    c.Level = slog.LevelDebug
    c.Format = slogx.FormatJSON
})
```

## Development

```bash
make test
make coverage
make lint
make pre-release        # full local gate (mirrors CI)
make pre-release-quick  # skip coverage + lint
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CHANGELOG.md](CHANGELOG.md).

## License

MIT — see [LICENSE](LICENSE).
