// Package main demonstrates Default / Dev / MustDefault presets.
package main

import (
	"context"
	"fmt"

	"github.com/KLORQ/quarshell-slogx"
)

func main() {
	// Default: JSON, Info, AddSource, OTel, stack on error, standard context keys.
	prod := slogx.Default()
	prod.InfoContext(context.Background(), "production preset")

	// Dev: Text, Debug, OTel, stack on error — convenient for local development.
	dev := slogx.Dev()
	dev.DebugContext(context.Background(), "development preset", "env", "local")

	// MustDefault sets slog.Default() to the production preset and returns it.
	global := slogx.MustDefault()
	fmt.Printf("global logger config format=%v level=%v\n",
		global.Config().Format, global.Config().Level)
}
