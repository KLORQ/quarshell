package slogx

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// ContextKey is a typed key for values stored in context.Context.
type ContextKey string

// String returns the key name used in log attributes.
func (k ContextKey) String() string {
	return string(k)
}

// FromContext extracts a Logger from context or returns a wrapper around slog.Default().
// The fallback supports Info/Error via slog.Default but SetLevel/UpdateConfig are no-ops
// unless the default logger was installed via SetupDefault/MustDefault (prefer ToContext).
func FromContext(ctx context.Context) *Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*Logger); ok && l != nil {
			return l
		}
	}
	if l, ok := slog.Default().Handler().(*DynamicHandler); ok && l != nil {
		// Recover slogx config when MustDefault/SetupDefault installed the handler.
		return &Logger{Logger: slog.Default(), cfgPtr: l.cfg, exitFunc: func(int) {}}
	}
	return &Logger{Logger: slog.Default()}
}

// ToContext injects a Logger into context. Nil logger is ignored.
func ToContext(ctx context.Context, l *Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

// WithValue stores a log attribute source in context under a typed key.
func WithValue(ctx context.Context, key ContextKey, val any) context.Context {
	return context.WithValue(ctx, key, val)
}

// Value reads a context value by typed key.
func Value(ctx context.Context, key ContextKey) any {
	return ctx.Value(key)
}
