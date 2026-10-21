// Package main demonstrates basic slogx setup: levels, formats, and helpers.
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/KLORQ/quarshell-slogx"
)

func main() {
	// Create a JSON logger at Debug so Trace and Info are both visible.
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithLevel(slog.LevelDebug),
	)

	ctx := context.Background()

	log.InfoContext(ctx, "application started", "service", "orders")
	log.TraceContext(ctx, "verbose diagnostic detail", "step", 1)

	// Err wraps an error as a consistent slog attribute.
	err := fmt.Errorf("timeout talking to database")
	log.ErrorContext(ctx, "operation failed", slogx.Err(err))

	// NewNop discards all output (useful in tests).
	nop := slogx.NewNop()
	nop.Info("this is discarded")
}
