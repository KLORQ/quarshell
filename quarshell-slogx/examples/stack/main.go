// Package main demonstrates stack traces and source location on Error/Fatal.
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/KLORQ/quarshell-slogx"
)

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithAddSource(true),
		slogx.WithStackOnError(true),
		slogx.WithStackDepth(16),
		// Override os.Exit so the example does not terminate the process.
		slogx.WithExitFunc(func(code int) {
			fmt.Printf("Fatal would exit with code %d\n", code)
		}),
	)

	ctx := context.Background()
	log.ErrorContext(ctx, "recoverable failure", slogx.Err(fmt.Errorf("connection reset")))
	log.FatalContext(ctx, "unrecoverable failure", slog.String("component", "bootstrap"))
}
