// Package main demonstrates typed context keys and logger injection.
package main

import (
	"context"
	"log/slog"

	"github.com/KLORQ/quarshell-slogx"
)

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithContextKeys(
			slogx.KeyRequestID,
			slogx.KeyUserID,
			slogx.KeyTenantID,
		),
	)

	ctx := context.Background()
	ctx = slogx.WithValue(ctx, slogx.KeyRequestID, "req-42")
	ctx = slogx.WithValue(ctx, slogx.KeyUserID, "user-7")
	ctx = slogx.WithValue(ctx, slogx.KeyTenantID, "tenant-acme")

	// Inject the logger into context for downstream helpers.
	ctx = slogx.ToContext(ctx, log)
	doWork(ctx)
}

func doWork(ctx context.Context) {
	// FromContext recovers the logger; falls back to slog.Default() if missing.
	log := slogx.FromContext(ctx)
	log.InfoContext(ctx, "work completed", slog.String("status", "ok"))
}
