// Package main demonstrates runtime log-level control.
package main

import (
	"fmt"
	"log/slog"

	"github.com/KLORQ/quarshell-slogx"
)

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithLevel(slog.LevelInfo),
		slogx.WithCorporateMasking(),
	)

	if err := log.SetLevelString("debug"); err != nil {
		panic(err)
	}
	fmt.Println("log level:", log.Level())

	for i := range 5 {
		log.Debug("tick", "i", i)
		log.Info("tick-info", "i", i, "token", "sk-live-example-secret")
	}
}
