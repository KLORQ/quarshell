// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/KLORQ/quarshell-core/openshell"
	"google.golang.org/protobuf/types/known/structpb"
)

var openAIKeyPattern = regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`)

// RegexBuiltinStage implements the pinned openshell/regex builtin in-process.
type RegexBuiltinStage struct {
	include    []string
	exclude    []string
	failClosed bool
}

func NewRegexBuiltinStage(config *structpb.Struct, include, exclude []string, failurePolicy ...bool) (*RegexBuiltinStage, error) {
	if err := openshell.ValidateRegexMiddlewareConfig(config); err != nil {
		return nil, err
	}
	failClosed := true
	if len(failurePolicy) > 0 {
		failClosed = failurePolicy[0]
	}
	return &RegexBuiltinStage{include: append([]string(nil), include...), exclude: append([]string(nil), exclude...), failClosed: failClosed}, nil
}

func (*RegexBuiltinStage) Name() string { return openshell.RegexMiddlewareName }

func (s *RegexBuiltinStage) Evaluate(_ context.Context, req Request) (Decision, error) {
	if !s.matchesHost(req.Host) {
		return Decision{Allow: true, Reason: "middleware selector skipped"}, nil
	}
	if uint64(len(req.Body)) > openshell.RegexMiddlewareMaxPayloadBytes {
		return s.httpFailure(fmt.Errorf("%s request body exceeds %d bytes", openshell.RegexMiddlewareName, openshell.RegexMiddlewareMaxPayloadBytes))
	}
	if !utf8.Valid(req.Body) {
		return s.httpFailure(fmt.Errorf("%s requires UTF-8 request bodies", openshell.RegexMiddlewareName))
	}
	replacement, findings, metadata := redactOpenAIKeys(string(req.Body))
	decision := Decision{Allow: true, Reason: "regex middleware allow", Findings: findings, Metadata: metadata}
	if replacement != string(req.Body) {
		decision.HasBody = true
		decision.Body = []byte(replacement)
	}
	return decision, nil
}

func (s *RegexBuiltinStage) httpFailure(err error) (Decision, error) {
	if s.failClosed {
		return Decision{}, err
	}
	return Decision{Allow: true, Reason: "regex middleware fail-open"}, nil
}

func (s *RegexBuiltinStage) OpenWebSocketSession(_ context.Context, req WebSocketRequest) (WebSocketSession, error) {
	return &regexBuiltinSession{stage: s, inspect: s.matchesHost(req.Host)}, nil
}

func (s *RegexBuiltinStage) matchesHost(host string) bool {
	if len(s.include) == 0 {
		for _, pattern := range s.exclude {
			if hostPatternMatch(pattern, host) {
				return false
			}
		}
		return true
	}
	included := false
	for _, pattern := range s.include {
		if hostPatternMatch(pattern, host) {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, pattern := range s.exclude {
		if hostPatternMatch(pattern, host) {
			return false
		}
	}
	return true
}

type regexBuiltinSession struct {
	stage   *RegexBuiltinStage
	inspect bool
}

func (s *regexBuiltinSession) Preflight(context.Context) (bool, error) { return s.inspect, nil }
func (*regexBuiltinSession) Start(context.Context, string) error       { return nil }
func (*regexBuiltinSession) Close(string) error                        { return nil }

func (s *regexBuiltinSession) Message(_ context.Context, _ uint64, direction WebSocketDirection, payload []byte, text bool) (WebSocketMessageDecision, error) {
	if !s.inspect || direction != WebSocketClientToUpstream {
		return WebSocketMessageDecision{Allow: true, Replacement: payload}, nil
	}
	if !text {
		return s.messageFailure(fmt.Errorf("%s supports only client-to-upstream WebSocket text messages", openshell.RegexMiddlewareName), payload)
	}
	if uint64(len(payload)) > openshell.RegexMiddlewareMaxPayloadBytes {
		return s.messageFailure(fmt.Errorf("%s WebSocket text message exceeds %d bytes", openshell.RegexMiddlewareName, openshell.RegexMiddlewareMaxPayloadBytes), payload)
	}
	if !utf8.Valid(payload) {
		return s.messageFailure(fmt.Errorf("%s requires UTF-8 WebSocket text messages", openshell.RegexMiddlewareName), payload)
	}
	replacement, findings, metadata := redactOpenAIKeys(string(payload))
	return WebSocketMessageDecision{Allow: true, Replacement: []byte(replacement), Findings: findings, Metadata: metadata}, nil
}

func (s *regexBuiltinSession) messageFailure(err error, payload []byte) (WebSocketMessageDecision, error) {
	if s.stage.failClosed {
		return WebSocketMessageDecision{}, err
	}
	return WebSocketMessageDecision{Allow: true, Replacement: payload}, nil
}

func redactOpenAIKeys(text string) (string, []Finding, map[string]string) {
	indices := openAIKeyPattern.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return text, nil, nil
	}
	return openAIKeyPattern.ReplaceAllString(text, "[REDACTED]"), []Finding{{
		Type: "regex.openai", Label: "openai regex match", Count: uint32(len(indices)), Confidence: "medium", Severity: "medium",
	}}, map[string]string{"regex_matches_replaced": fmt.Sprintf("%d", len(indices))}
}
