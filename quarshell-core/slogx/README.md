# slogx

`quarshell-core/slogx` contains structured logging helpers for `log/slog`,
including redaction, trace context, level control, and bounded payload dumps.
The package was absorbed into `quarshell-core`; its existing MIT terms remain
in [LICENSE](LICENSE).

```go
logger := slogx.New(slogx.Options{Level: slog.LevelInfo})
```

The package is imported as `github.com/KLORQ/quarshell-core/slogx`.

MIT — see [LICENSE](LICENSE).
