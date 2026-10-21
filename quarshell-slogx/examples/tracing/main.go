// Package main demonstrates OpenTelemetry trace_id / span_id injection.
package main

import (
	"context"
	"log/slog"

	"github.com/KLORQ/quarshell-slogx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tr := tp.Tracer("examples/tracing")

	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithTraceContext(true),
		// Optional: rename OTel attribute keys in log output.
		slogx.WithTraceKeys("trace_id", "span_id", "trace_sampled"),
	)

	ctx, span := tr.Start(context.Background(), "handleRequest")
	defer span.End()

	// Logs automatically include trace_id and span_id from the active span.
	log.InfoContext(ctx, "request handled", slog.String("route", "/orders"))
}
