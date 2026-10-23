package slogx

import (
	"log/slog"
	"strings"
)

// LevelNames maps slog.Level values to custom string labels in log output.
type LevelNames map[slog.Level]string

const (
	// LevelTrace is below slog.LevelDebug for verbose diagnostics.
	LevelTrace = slog.Level(-8)
	// LevelFatal is above slog.LevelError for unrecoverable failures.
	LevelFatal = slog.Level(12)
)

var defaultLevelNames = LevelNames{
	LevelTrace: "TRACE",
	LevelFatal: "FATAL",
}

func getLevelName(l slog.Level, customNames LevelNames) string {
	if name, ok := customNames[l]; ok {
		return name
	}
	return strings.ToUpper(l.String())
}
