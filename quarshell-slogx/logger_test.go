package slogx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestLogger_UpdateConfig(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatJSON), WithLevel(slog.LevelInfo))

	l.Info("message 1")
	assert.Contains(t, buf.String(), "message 1")
	buf.Reset()

	l.UpdateConfig(func(c *Config) { c.Level = slog.LevelError })
	l.Info("invisible")
	assert.Empty(t, buf.String())
	l.Error("visible error")
	assert.Contains(t, buf.String(), "visible error")
}

func TestLogger_FormatSwitch(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatText))

	l.Info("text mode")
	assert.Contains(t, buf.String(), "level=INFO")
	buf.Reset()

	l.UpdateConfig(func(c *Config) { c.Format = FormatJSON })
	l.Info("json mode")
	assert.Contains(t, buf.String(), `"level":"INFO"`)
	assert.True(t, json.Valid(buf.Bytes()))
}

func TestLogger_WithAndWithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatJSON)).With("svc", "orders").WithGroup("meta")
	l.Info("ok", "region", "eu")
	out := buf.String()
	assert.Contains(t, out, `"svc":"orders"`)
	assert.Contains(t, out, `"meta"`)
}

func TestLogger_TraceAndLevels(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatJSON), WithLevel(LevelTrace))
	l.TraceContext(context.Background(), "trace-msg")
	assert.Contains(t, buf.String(), `"level":"TRACE"`)
}

func TestTraceContextInjection(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tr := tp.Tracer("test")

	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatJSON), WithTraceContext(true))

	ctx, span := tr.Start(context.Background(), "op")
	defer span.End()

	l.InfoContext(ctx, "hello")
	out := buf.String()
	assert.Contains(t, out, `"trace_id"`)
	assert.Contains(t, out, `"span_id"`)
}

func TestStackOnError(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatJSON), WithStackOnError(true), WithAddSource(true))
	l.Error("boom")
	assert.Contains(t, buf.String(), `"stack"`)
	assert.Contains(t, buf.String(), `"source"`)
}

func TestContextKeys(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(WithOutput(buf), WithFormat(FormatJSON), WithContextKeys(KeyRequestID))
	ctx := WithValue(context.Background(), KeyRequestID, "req-42")
	l.InfoContext(ctx, "ok")
	assert.Contains(t, buf.String(), `"request_id":"req-42"`)
}

func TestFromContext_ToContext(t *testing.T) {
	l := New(WithOutput(io.Discard))
	ctx := ToContext(context.Background(), l)
	got := FromContext(ctx)
	assert.Same(t, l, got)

	fallback := FromContext(context.Background())
	assert.NotNil(t, fallback)
}

func TestMaskingAndRemoval(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(
		WithOutput(buf),
		WithFormat(FormatJSON),
		WithMaskRules(NewMaskRules().Add("email", MaskEmail).Add("secret", MaskSecret)),
		WithRemoval(NewRemovalSet("password")),
	)
	l.Info("login",
		slog.String("email", "alice@example.com"),
		slog.String("secret", "top"),
		slog.String("password", "hide-me"),
	)
	out := buf.String()
	assert.Contains(t, out, "al***@example.com")
	assert.Contains(t, out, "[SECRET]")
	assert.NotContains(t, out, "hide-me")
	assert.NotContains(t, out, `"password"`)
}

func TestDumpStruct(t *testing.T) {
	type Address struct{ City string }
	type User struct {
		Name    string
		Email   string
		Address Address
	}
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{"email": MaskEmail}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	u := User{Name: "Alice", Email: "alice@example.com", Address: Address{City: "Berlin"}}
	out := Serialize(cfg, u)
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Alice", m["Name"])
	assert.Equal(t, "al***@example.com", m["Email"])
	addr, ok := m["Address"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Berlin", addr["City"])
}

func TestDumpCycle(t *testing.T) {
	type Node struct{ Next *Node }
	n := &Node{}
	n.Next = n
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	out := Serialize(cfg, n)
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "[cycle]", m["Next"])
}

func TestDumpDepthAndMaxFields(t *testing.T) {
	type Inner struct{ V string }
	type Outer struct {
		A Inner
		B Inner
		C Inner
	}
	cfg := Config{
		DumpMaxDepth: 1, DumpMaxFields: 2,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	out := Serialize(cfg, Outer{A: Inner{V: "a"}, B: Inner{V: "b"}, C: Inner{V: "c"}})
	assert.NotNil(t, out)
}

func TestDumpAttrs(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	SetGlobalDumpConfig(&cfg)
	attr := Dump("obj", map[string]any{"k": "v"})
	assert.Equal(t, "obj", attr.Key)

	group := DumpGroup("g", map[string]any{"k": "v"})
	assert.Equal(t, "g", group.Key)
}

func TestHTTPRequestSanitizesHeaders(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	SetGlobalDumpConfig(&cfg)

	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/v1/users", strings.NewReader(`{"name":"Alice"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")

	attr := HTTPRequest("request", req)
	val := attr.Value.Any().(map[string]any)
	headers := val["headers"].(map[string]any)
	assert.Equal(t, "[SECRET]", headers["Authorization"])
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, `{"name":"Alice"}`, val["body"])

	// Body must be restored for downstream readers.
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"name":"Alice"}`, string(body))
}

func TestHTTPResponseSanitize(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	SetGlobalDumpConfig(&cfg)
	resp := &http.Response{
		Status: "200 OK",
		Header: http.Header{"Set-Cookie": []string{"a=b"}, "X-Req": []string{"1"}},
		Body:   io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	attr := HTTPResponse("response", resp)
	val := attr.Value.Any().(map[string]any)
	headers := val["headers"].(map[string]any)
	assert.Equal(t, "[SECRET]", headers["Set-Cookie"])
	assert.Equal(t, "1", headers["X-Req"])
}

func TestGRPCCallSanitizesMetadata(t *testing.T) {
	cfg := Config{
		DumpMaxDepth: 5, DumpMaxFields: 64,
		MaskKeys: MaskMap{}, RemoveKeys: RemoveMap{}, Masker: &DefaultMasker{},
	}
	SetGlobalDumpConfig(&cfg)

	type CreateUserRequest struct{ Email string }
	attr := GRPCCall("grpc", "/users.UserService/Create",
		CreateUserRequest{Email: "bob@example.com"},
		Metadata{"authorization": {"Bearer grpc-secret"}, "x-request-id": {"req-1"}},
	)
	val := attr.Value.Any().(map[string]any)
	meta := val["metadata"].(map[string]any)
	assert.Equal(t, "[SECRET]", meta["authorization"])
	assert.Equal(t, "req-1", meta["x-request-id"])
}

func TestGRPCResponseAndStream(t *testing.T) {
	attr := GRPCResponse("resp", map[string]any{"id": "1"})
	assert.Equal(t, "resp", attr.Key)
	stream := GRPCStreamEvent("ev", "/svc/Watch", "recv", map[string]any{"n": 1}, nil)
	assert.Equal(t, "ev", stream.Key)
}

func TestFatalUsesExitFunc(t *testing.T) {
	var code int
	l := New(WithOutput(&bytes.Buffer{}), WithExitFunc(func(c int) { code = c }))
	l.FatalContext(context.Background(), "fatal")
	assert.Equal(t, 1, code)
}

func TestErrHelper(t *testing.T) {
	assert.Equal(t, "nil", Err(nil).Value.String())
	assert.Equal(t, "boom", Err(fmt.Errorf("boom")).Value.String())
}

func TestParseFormat(t *testing.T) {
	assert.Equal(t, FormatJSON, ParseFormat("json"))
	assert.Equal(t, FormatJSON, ParseFormat("JSON"))
	assert.Equal(t, FormatText, ParseFormat("text"))
}

func TestPresets(t *testing.T) {
	d := Default()
	require.NotNil(t, d)
	assert.Equal(t, FormatJSON, d.Config().Format)
	assert.True(t, d.Config().TraceContext)
	assert.True(t, d.Config().StackOnError)

	dev := Dev()
	assert.Equal(t, FormatText, dev.Config().Format)
	assert.Equal(t, slog.LevelDebug, dev.Config().Level)
}

func TestGetLevelName(t *testing.T) {
	assert.Equal(t, "TRACE", getLevelName(LevelTrace, defaultLevelNames))
	assert.Equal(t, "FATAL", getLevelName(LevelFatal, defaultLevelNames))
	assert.Equal(t, "INFO", getLevelName(slog.LevelInfo, nil))
}

func TestRemovalSet(t *testing.T) {
	set := NewRemovalSet("a").Add("b", "c")
	assert.Equal(t, []string{"a", "b", "c"}, set.Keys())
}

func TestHandler_Enabled(t *testing.T) {
	l := New(WithOutput(io.Discard), WithLevel(slog.LevelWarn))
	h := l.Handler()
	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelError))
}

func TestNilHTTPAndGRPC(t *testing.T) {
	assert.Equal(t, "nil", HTTPRequest("r", nil).Value.String())
	assert.Equal(t, "nil", HTTPResponse("r", nil).Value.String())
}

func TestSerializeDefault(t *testing.T) {
	out := SerializeDefault(map[string]any{"x": 1})
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, m["x"])
}
