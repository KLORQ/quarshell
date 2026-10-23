// Package main demonstrates runtime config updates (level / format) without restart.
package main

import (
	"context"
	"log/slog"

	"github.com/KLORQ/quarshell-core/slogx"
)

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatText),
		slogx.WithLevel(slog.LevelInfo),
	)

	ctx := context.Background()
	log.InfoContext(ctx, "text mode active")

	// Atomically switch format and raise the level threshold.
	log.UpdateConfig(func(c *slogx.Config) {
		c.Format = slogx.FormatJSON
		c.Level = slog.LevelWarn
	})

	log.InfoContext(ctx, "this Info is skipped after UpdateConfig")
	log.WarnContext(ctx, "warn is visible in JSON")

	// SetLevel is a convenience wrapper around UpdateConfig.
	log.SetLevel(slog.LevelDebug)
	log.DebugContext(ctx, "debug is visible again")
}
