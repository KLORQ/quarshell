package slogx

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

func traceAttrs(ctx context.Context, cfg *Config) []traceAttr {
	if !cfg.TraceContext {
		return nil
	}
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return nil
	}
	out := make([]traceAttr, 0, 3)
	out = append(out, traceAttr{cfg.TraceIDKey, sc.TraceID().String()})
	out = append(out, traceAttr{cfg.SpanIDKey, sc.SpanID().String()})
	if sc.IsSampled() {
		out = append(out, traceAttr{cfg.TraceSampleKey, true})
	}
	return out
}

type traceAttr struct {
	key string
	val any
}
