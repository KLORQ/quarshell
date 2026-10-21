package slogx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDumpOptionsAndSliceMap(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}

	type Item struct {
		Name  string
		Tags  []string
		Meta  map[string]string
		Extra *Item
	}
	item := Item{
		Name: "a",
		Tags: []string{"x", "y"},
		Meta: map[string]string{"k": "v", "password": "secret"},
	}

	out := Serialize(cfg, item,
		WithDumpDepth(4),
		WithDumpMaxFields(100),
		WithDumpMaskKeys(MaskMap{"Name": MaskSecret}),
		WithDumpRemoval("password"),
	)
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "[SECRET]", m["Name"])
	tags, ok := m["Tags"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"x", "y"}, tags)
	meta, ok := m["Meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "v", meta["k"])
	_, hasPassword := meta["password"]
	assert.False(t, hasPassword)
	assert.Nil(t, m["Extra"])
}

func TestDumpPointerAndNil(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 3, DumpMaxFields: 10,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	assert.Nil(t, Serialize(cfg, nil))

	type Box struct{ V int }
	b := &Box{V: 7}
	out := Serialize(cfg, b)
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 7, m["V"])
}

func TestOptionsCoverage(t *testing.T) {
	l := New(
		WithOutput(io.Discard),
		WithMaskKey("token", MaskSecret),
		WithMaskKeys(MaskMap{"pin": MaskSecret}),
		WithMasker(&DefaultMasker{}),
		WithLevelNames(LevelNames{slog.LevelInfo: "INFORMATION"}),
		WithStackDepth(8),
		WithTraceKeys("tid", "sid", "sampled"),
		WithDumpLimits(3, 32),
		WithExitFunc(func(int) {}),
	)
	cfg := l.Config()
	assert.Equal(t, MaskSecret, cfg.MaskKeys["token"])
	assert.Equal(t, MaskSecret, cfg.MaskKeys["pin"])
	assert.Equal(t, "INFORMATION", cfg.LevelNames[slog.LevelInfo])
	assert.Equal(t, 8, cfg.StackDepth)
	assert.Equal(t, "tid", cfg.TraceIDKey)
	assert.Equal(t, 3, cfg.DumpMaxDepth)
	assert.Equal(t, 32, cfg.DumpMaxFields)

	l.SetLevel(slog.LevelError)
	assert.Equal(t, slog.LevelError, l.Config().Level)

	assert.NotNil(t, NewNop())
	assert.NotNil(t, SetupDefault(WithOutput(io.Discard)))
	assert.NotNil(t, MustDefault())

	rules := NewMaskRules().Add("e", MaskEmail)
	assert.Equal(t, MaskEmail, rules.Keys()["e"])
	assert.Equal(t, "[MASKED]", (&DefaultMasker{}).Mask("x", MaskDefault))
	assert.Equal(t, "***", maskPhone("123"))
	assert.Equal(t, "****", maskCard("1234"))

	assert.Nil(t, Value(context.Background(), KeyRequestID))
	ctx := WithValue(context.Background(), KeyRequestID, "r")
	assert.Equal(t, "r", Value(ctx, KeyRequestID))
}

func TestSanitizeMetadataAndHeadersExtras(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys:   MaskMap{"x-email": MaskEmail},
		RemoveKeys: RemoveMap{"x-skip": {}},
		Masker:     &DefaultMasker{},
	}
	md := sanitizeMetadata(Metadata{
		"x-email": {"alice@example.com"},
		"x-skip":  {"nope"},
		"x-multi": {"a", "b"},
	}, cfg)
	assert.Equal(t, "al***@example.com", md["x-email"].([]string)[0])
	_, ok := md["x-skip"]
	assert.False(t, ok)
	assert.Equal(t, []string{"a", "b"}, md["x-multi"])

	headers := sanitizeHeaders(http.Header{
		"X-Email": []string{"bob@example.com"},
		"X-Skip":  []string{"nope"},
		"X-Multi": []string{"1", "2"},
	}, cfg)
	assert.Equal(t, []string{"bo***@example.com"}, headers["X-Email"])
	_, ok = headers["X-Skip"]
	assert.False(t, ok)
	assert.Equal(t, []string{"1", "2"}, headers["X-Multi"])

	body, truncated := readBody(nil, 0)
	assert.Equal(t, "", body)
	assert.False(t, truncated)
	RestoreBody(nil, "")
}

func TestMaskPhoneCardEdges(t *testing.T) {
	assert.Equal(t, "791*****456", maskPhone("+7 911 222 3456"))
	assert.Equal(t, "4276 **** **** 0000", maskCard("4276 1234 5678 0000"))
	assert.Equal(t, "***@***", maskEmail("not-an-email"))
}
