// Package main demonstrates data masking and field removal.
package main

import (
	"context"
	"log/slog"

	"github.com/KLORQ/quarshell-core/slogx"
)

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithMaskRules(
			slogx.NewMaskRules().
				Add("email", slogx.MaskEmail).
				Add("phone", slogx.MaskPhone).
				Add("card", slogx.MaskCard).
				Add("api_key", slogx.MaskSecret),
		),
		slogx.WithRemoval(
			slogx.NewRemovalSet("password", "session_token"),
		),
	)

	ctx := context.Background()
	log.InfoContext(ctx, "user login",
		slog.String("email", "alice@example.com"),
		slog.String("phone", "+79111234567"),
		slog.String("card", "4276123456780000"),
		slog.String("api_key", "sk-live-secret"),
		slog.String("password", "must-not-appear"),
		slog.String("session_token", "tok-xyz"),
	)
}
