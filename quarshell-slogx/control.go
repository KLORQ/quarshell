package slogx

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// ParseLevel converts common level strings to slog.Level.
// Accepts: trace, debug, info, warn/warning, error, fatal (case-insensitive),
// or a numeric slog.Level value.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		if n, err := strconv.Atoi(s); err == nil {
			return slog.Level(n), nil
		}
		return 0, fmt.Errorf("slogx: unknown level %q", s)
	}
}

// SetLevelString parses and applies a level string.
func (l *Logger) SetLevelString(s string) error {
	lvl, err := ParseLevel(s)
	if err != nil {
		return err
	}
	l.SetLevel(lvl)
	return nil
}

// WatchLevelEnv polls an environment variable and applies level changes.
// Stops when ctx is canceled. Empty envKey defaults to LOG_LEVEL;
// empty interval defaults to 2s.
func (l *Logger) WatchLevelEnv(ctx context.Context, envKey string, interval time.Duration) {
	if envKey == "" {
		envKey = "LOG_LEVEL"
	}
	if interval <= 0 {
		interval = defaultLevelPollInterval
	}
	var last string
	apply := func() {
		v := strings.TrimSpace(os.Getenv(envKey))
		if v == "" || v == last {
			return
		}
		if err := l.SetLevelString(v); err == nil {
			last = v
		}
	}
	apply()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			apply()
		}
	}
}
