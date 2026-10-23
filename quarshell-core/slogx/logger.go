package slogx

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
)

// Logger wraps slog.Logger with atomic runtime configuration updates.
type Logger struct {
	*slog.Logger
	cfgPtr   *atomic.Pointer[Config]
	exitFunc func(int)
}

// New creates a Logger with the provided options.
func New(opts ...Option) *Logger {
	o := defaultOptions()
	for _, fn := range opts {
		if fn != nil {
			fn(o)
		}
	}

	ptr := &atomic.Pointer[Config]{}
	ptr.Store(o.initialConfig)

	handler := &DynamicHandler{cfg: ptr}
	return &Logger{
		Logger:   slog.New(handler),
		cfgPtr:   ptr,
		exitFunc: o.exitFunc,
	}
}

// Config returns a snapshot of the current logger configuration.
func (l *Logger) Config() Config {
	if l == nil || l.cfgPtr == nil {
		return Config{Level: slog.LevelInfo, Masker: &DefaultMasker{}}
	}
	cfg := l.cfgPtr.Load()
	if cfg == nil {
		return Config{Level: slog.LevelInfo, Masker: &DefaultMasker{}}
	}
	return *cfg
}

// With returns a derived Logger sharing the same config pointer.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger:   l.Logger.With(args...),
		cfgPtr:   l.cfgPtr,
		exitFunc: l.exitFunc,
	}
}

// WithGroup returns a grouped Logger sharing the same config pointer.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger:   l.Logger.WithGroup(name),
		cfgPtr:   l.cfgPtr,
		exitFunc: l.exitFunc,
	}
}

// UpdateConfig atomically updates logger configuration using copy-on-write.
// No-op when the logger was constructed without a config pointer (e.g. FromContext fallback).
func (l *Logger) UpdateConfig(fn func(*Config)) {
	if l == nil || l.cfgPtr == nil || fn == nil {
		return
	}
	oldCfg := l.cfgPtr.Load()
	if oldCfg == nil {
		return
	}
	newCfg := oldCfg.Clone()
	fn(newCfg)
	l.cfgPtr.Store(newCfg)
}

// SetLevel updates the logging threshold.
func (l *Logger) SetLevel(lvl slog.Level) {
	l.UpdateConfig(func(c *Config) {
		c.Level = lvl
	})
}

// Level returns the current logging threshold.
func (l *Logger) Level() slog.Level {
	if l == nil || l.cfgPtr == nil {
		return slog.LevelInfo
	}
	cfg := l.cfgPtr.Load()
	if cfg == nil {
		return slog.LevelInfo
	}
	return cfg.Level
}

// TraceContext logs at LevelTrace.
func (l *Logger) TraceContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, LevelTrace, msg, args...)
}

// FatalContext logs at LevelFatal and exits the process.
func (l *Logger) FatalContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, LevelFatal, msg, args...)
	l.exitFunc(1)
}

// SetupDefault initializes a Logger and sets it as slog.Default().
func SetupDefault(opts ...Option) *Logger {
	log := New(opts...)
	slog.SetDefault(log.Logger)
	return log
}

// NewNop creates a logger that discards all output.
func NewNop() *Logger {
	return New(WithOutput(io.Discard))
}

// Err creates a structured error attribute.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.String("error", "nil")
	}
	return slog.String("error", err.Error())
}

// Standard context keys for enterprise services.
var (
	KeyRequestID = ContextKey("request_id")
	KeyUserID    = ContextKey("user_id")
	KeyTenantID  = ContextKey("tenant_id")
)

// Default returns a production-ready logger: JSON, Info level, OTel, stack on error,
// and corporate field masking.
func Default() *Logger {
	return New(
		WithFormat(FormatJSON),
		WithLevel(slog.LevelInfo),
		WithAddSource(true),
		WithTraceContext(true),
		WithStackOnError(true),
		WithCorporateMasking(),
		WithContextKeys(KeyRequestID, KeyUserID, KeyTenantID),
	)
}

// Dev returns a text logger suitable for local development.
func Dev() *Logger {
	return New(
		WithFormat(FormatText),
		WithLevel(slog.LevelDebug),
		WithTraceContext(true),
		WithStackOnError(true),
		WithCorporateMasking(),
		WithContextKeys(KeyRequestID),
	)
}

// Corporate is Default() with Fingerprint-enabled corporate masking (alias for clarity).
func Corporate() *Logger {
	return Default()
}

// MustDefault sets Default() as slog.Default() and returns it.
func MustDefault() *Logger {
	return SetupDefault(
		WithFormat(FormatJSON),
		WithLevel(slog.LevelInfo),
		WithAddSource(true),
		WithTraceContext(true),
		WithStackOnError(true),
		WithCorporateMasking(),
		WithContextKeys(KeyRequestID, KeyUserID, KeyTenantID),
	)
}
