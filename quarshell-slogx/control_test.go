package slogx

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	lvl, err := ParseLevel("debug")
	require.NoError(t, err)
	assert.Equal(t, slog.LevelDebug, lvl)
	_, err = ParseLevel("nope")
	assert.Error(t, err)

	for _, s := range []string{"trace", "info", "warn", "warning", "error", "fatal", "0"} {
		_, err := ParseLevel(s)
		require.NoError(t, err, s)
	}
}

func TestSetLevelString(t *testing.T) {
	l := New(WithOutput(io.Discard), WithLevel(slog.LevelInfo))
	require.NoError(t, l.SetLevelString("debug"))
	assert.Equal(t, slog.LevelDebug, l.Level())
}

func TestWatchLevelEnv(t *testing.T) {
	l := New(WithOutput(io.Discard), WithLevel(slog.LevelInfo))
	ctx := t.Context()
	t.Setenv("SLOGX_TEST_LEVEL", "error")
	go l.WatchLevelEnv(ctx, "SLOGX_TEST_LEVEL", 50*time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.Level() == slog.LevelError {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("level not updated: %v", l.Level())
}

func TestCorporateMaskingOption(t *testing.T) {
	l := New(WithCorporateMasking(), WithOutput(io.Discard))
	_, ok := l.Config().Masker.(*CorporateMasker)
	assert.True(t, ok)
	_, ok = l.Config().MaskKeys["password"]
	assert.True(t, ok)
}
