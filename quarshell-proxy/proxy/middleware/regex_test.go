package middleware

import (
	"context"
	"strings"
	"testing"

	"github.com/KLORQ/quarshell-proto/compat/openshell"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestRegexBuiltinHTTPRequest(t *testing.T) {
	stage, err := NewRegexBuiltinStage(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := stage.Evaluate(context.Background(), Request{Host: "api.example", Body: []byte("input sk-12345678901234567890 and sk-abcdefghijklmnop")})
	if err != nil || !decision.Allow || !decision.HasBody || string(decision.Body) != "input [REDACTED] and [REDACTED]" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	if len(decision.Findings) != 1 || decision.Findings[0].Type != "regex.openai" || decision.Findings[0].Count != 2 || decision.Findings[0].Confidence != "medium" || decision.Findings[0].Severity != "medium" || decision.Metadata["regex_matches_replaced"] != "2" {
		t.Fatalf("diagnostics=%+v metadata=%v", decision.Findings, decision.Metadata)
	}
	noMatch, err := stage.Evaluate(context.Background(), Request{Host: "api.example", Body: []byte("ordinary request")})
	if err != nil || !noMatch.Allow || noMatch.HasBody || len(noMatch.Findings) != 0 || len(noMatch.Metadata) != 0 {
		t.Fatalf("no-match decision=%+v err=%v", noMatch, err)
	}
	keywords, err := stage.Evaluate(context.Background(), Request{Host: "api.example", Body: []byte(`{"password":"alpha beta","secret":"alpha,beta","api_key":"alpha\\\"beta"}\npassword=alpha\nnotpassword=omega`)})
	if err != nil || !keywords.Allow || keywords.HasBody || len(keywords.Findings) != 0 {
		t.Fatalf("keyword-like values were unexpectedly rewritten: %+v err=%v", keywords, err)
	}
}

func TestRegexBuiltinRejectsInvalidAndOversizedHTTPBody(t *testing.T) {
	stage, err := NewRegexBuiltinStage(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Evaluate(context.Background(), Request{Body: []byte{0xff}}); err == nil {
		t.Fatal("invalid UTF-8 body was accepted")
	}
	if _, err := stage.Evaluate(context.Background(), Request{Body: make([]byte, openshell.RegexMiddlewareMaxPayloadBytes+1)}); err == nil {
		t.Fatal("oversized body was accepted")
	}
	boundary := []byte(strings.Repeat("x", int(openshell.RegexMiddlewareMaxPayloadBytes)))
	decision, err := stage.Evaluate(context.Background(), Request{Body: boundary})
	if err != nil || !decision.Allow || decision.HasBody {
		t.Fatalf("payload at the 256 KiB boundary was rejected or mutated: %+v err=%v", decision, err)
	}
}

func TestRegexBuiltinFailurePolicy(t *testing.T) {
	stage, err := NewRegexBuiltinStage(nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := stage.Evaluate(context.Background(), Request{Body: []byte{0xff}})
	if err != nil || !decision.Allow || decision.Reason != "regex middleware fail-open" || decision.HasBody {
		t.Fatalf("fail-open HTTP decision=%+v err=%v", decision, err)
	}
	session, err := stage.OpenWebSocketSession(context.Background(), WebSocketRequest{})
	if err != nil {
		t.Fatal(err)
	}
	message, err := session.Message(context.Background(), 1, WebSocketClientToUpstream, []byte{0xff}, true)
	if err != nil || !message.Allow || string(message.Replacement) != string([]byte{0xff}) {
		t.Fatalf("fail-open WS decision=%+v err=%v", message, err)
	}
}

func TestRegexBuiltinConfigAndSelectors(t *testing.T) {
	for _, raw := range []map[string]any{{"mode": "allow"}, {"mode": "redact", "pattern": "secret"}, {"mode": 42}} {
		config, err := structpb.NewStruct(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := NewRegexBuiltinStage(config, nil, nil); err == nil {
			t.Fatalf("invalid config accepted: %v", raw)
		}
	}
	valid, err := structpb.NewStruct(map[string]any{"mode": "redact"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewRegexBuiltinStage(valid, nil, nil); err != nil {
		t.Fatalf("explicit redact mode rejected: %v", err)
	}
	stage, err := NewRegexBuiltinStage(nil, []string{"*.example.com"}, []string{"private.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := stage.Evaluate(context.Background(), Request{Host: "private.example.com", Body: []byte("ordinary")})
	if err != nil || !skipped.Allow || skipped.Reason != "middleware selector skipped" {
		t.Fatalf("selector decision=%+v err=%v", skipped, err)
	}
}

func TestRegexBuiltinWebSocketDirectionAndLimits(t *testing.T) {
	stage, err := NewRegexBuiltinStage(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := stage.OpenWebSocketSession(context.Background(), WebSocketRequest{Host: "api.example"})
	if err != nil {
		t.Fatal(err)
	}
	inspect, err := session.Preflight(context.Background())
	if err != nil || !inspect {
		t.Fatalf("inspect=%v err=%v", inspect, err)
	}
	key := []byte("sk-12345678901234567890")
	client, err := session.Message(context.Background(), 1, WebSocketClientToUpstream, key, true)
	if err != nil || !client.Allow || string(client.Replacement) != "[REDACTED]" || len(client.Findings) != 1 || client.Metadata["regex_matches_replaced"] != "1" {
		t.Fatalf("client decision=%+v err=%v", client, err)
	}
	payload := []byte(`{"type":"response.create","input":"sk-ABCDEFGHIJKLMNOP sk-QRSTUVWXYZabcdef"}`)
	redacted, err := session.Message(context.Background(), 8, WebSocketClientToUpstream, payload, true)
	if err != nil || string(redacted.Replacement) != `{"type":"response.create","input":"[REDACTED] [REDACTED]"}` || len(redacted.Findings) != 1 || redacted.Findings[0].Count != 2 || redacted.Metadata["regex_matches_replaced"] != "2" {
		t.Fatalf("multi-match WebSocket result=%+v err=%v", redacted, err)
	}
	noMatch, err := session.Message(context.Background(), 7, WebSocketClientToUpstream, []byte(`{"type":"response.create","input":"public"}`), true)
	if err != nil || !noMatch.Allow || len(noMatch.Findings) != 0 || len(noMatch.Metadata) != 0 || string(noMatch.Replacement) != `{"type":"response.create","input":"public"}` {
		t.Fatalf("WebSocket no-match decision=%+v err=%v", noMatch, err)
	}
	response, err := session.Message(context.Background(), 2, WebSocketUpstreamToClient, key, true)
	if err != nil || !response.Allow || string(response.Replacement) != string(key) || len(response.Findings) != 0 {
		t.Fatalf("response changed: %+v err=%v", response, err)
	}
	if _, err := session.Message(context.Background(), 3, WebSocketClientToUpstream, key, false); err == nil {
		t.Fatal("binary WebSocket message was accepted")
	}
	if _, err := session.Message(context.Background(), 4, WebSocketClientToUpstream, []byte{0xff}, true); err == nil {
		t.Fatal("invalid UTF-8 WebSocket text was accepted")
	}
	if _, err := session.Message(context.Background(), 5, WebSocketClientToUpstream, []byte(strings.Repeat("x", int(openshell.RegexMiddlewareMaxPayloadBytes)+1)), true); err == nil {
		t.Fatal("oversized WebSocket text was accepted")
	}
	boundary := []byte(strings.Repeat("x", int(openshell.RegexMiddlewareMaxPayloadBytes)))
	message, err := session.Message(context.Background(), 6, WebSocketClientToUpstream, boundary, true)
	if err != nil || !message.Allow || string(message.Replacement) != string(boundary) {
		t.Fatalf("WebSocket payload at the 256 KiB boundary failed: decision=%+v err=%v", message, err)
	}
}

func TestFromEnvironRegistersRegexBuiltinWithoutEndpoint(t *testing.T) {
	pipeline := FromEnviron([]string{`QUARSHELL_SUPERVISOR_MIDDLEWARES=[{"name":"openshell/regex","config":{"mode":"redact"}}]`})
	if pipeline == nil || len(pipeline.Stages) != 1 || pipeline.Stages[0].Name() != openshell.RegexMiddlewareName {
		t.Fatalf("pipeline=%+v", pipeline)
	}
	decision, err := pipeline.Run(context.Background(), Request{Host: "api.example", Body: []byte("sk-12345678901234567890")})
	if err != nil || !decision.Allow || string(decision.Body) != "[REDACTED]" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	noMatch, err := pipeline.Run(context.Background(), Request{Host: "api.example", Body: []byte("ordinary request")})
	if err != nil || !noMatch.Allow || noMatch.HasBody || len(noMatch.Findings) != 0 {
		t.Fatalf("no-match pipeline decision=%+v err=%v", noMatch, err)
	}
}
