// SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
// SPDX-License-Identifier: Apache-2.0

package logbuf

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHubRemoveDropsBuffer(t *testing.T) {
	h := NewHub(8)
	h.Append("demo", []Line{{TS: time.Now().UTC(), Source: "proxy", Level: "INFO", Text: "hi"}})
	if len(h.Names()) != 1 {
		t.Fatalf("Names=%v want [demo]", h.Names())
	}
	h.Remove("demo")
	if len(h.Names()) != 0 {
		t.Fatalf("after Remove Names=%v want empty", h.Names())
	}
	if got := h.Snapshot("demo", time.Time{}, "", "", 0); len(got) != 0 {
		t.Fatalf("Snapshot after Remove len=%d want 0", len(got))
	}
}

func TestHubRemoveIdempotent(t *testing.T) {
	h := NewHub(4)
	h.Remove("")
	h.Remove("missing")
	var nilHub *Hub
	nilHub.Remove("x")
}

func TestHubRingCaps(t *testing.T) {
	h := NewHub(3)
	for range 10 {
		h.Append("s", []Line{{Text: "x"}})
	}
	got := h.Snapshot("s", time.Time{}, "", "", 0)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestHubBoundsObservationMemoryAndReportsCursorGaps(t *testing.T) {
	h := NewHub(100)
	for i := range 32 {
		h.Append("s", []Line{{Text: fmt.Sprintf("%02d:%s", i, strings.Repeat("x", 60<<10))}})
	}
	lines, cursor := h.Tail("s", 0)
	if len(lines) == 0 || len(lines) >= 32 {
		t.Fatalf("byte budget did not evict old observations: retained=%d", len(lines))
	}
	if got := lines[len(lines)-1].Sequence; got != cursor || cursor != 32 {
		t.Fatalf("last sequence=%d cursor=%d, want 32", got, cursor)
	}
	if _, latest, gap := h.ReadAfter("s", 0); !gap || latest != cursor {
		t.Fatalf("expired cursor gap=%v latest=%d, want gap=true latest=%d", gap, latest, cursor)
	}
	if after, _, gap := h.ReadAfter("s", cursor); gap || len(after) != 0 {
		t.Fatalf("current cursor read returned %d lines, gap=%v", len(after), gap)
	}
}

func TestBoundLineCapsPayloadAndFieldsDeterministically(t *testing.T) {
	fields := make(map[string]string, maxLineFields+1)
	for i := range maxLineFields + 1 {
		fields[fmt.Sprintf("field-%03d", i)] = "value"
	}
	line := boundLine(Line{
		Source: strings.Repeat("s", 1024),
		Level:  strings.Repeat("l", 1024),
		Target: strings.Repeat("t", 1024),
		Text:   "event",
		Fields: fields,
	})
	gotBytes := len(line.Source) + len(line.Level) + len(line.Target) + len(line.Text)
	for key, value := range line.Fields {
		gotBytes += len(key) + len(value)
	}
	if gotBytes > maxLineBytes {
		t.Fatalf("payload bytes=%d exceed line budget %d", gotBytes, maxLineBytes)
	}
	if len(line.Fields) != maxLineFields {
		t.Fatalf("fields=%d, want capped at %d", len(line.Fields), maxLineFields)
	}
	if _, ok := line.Fields["field-000"]; !ok {
		t.Fatal("deterministic field selection omitted lexicographically first field")
	}
	if _, ok := line.Fields["field-128"]; ok {
		t.Fatal("field cap retained an entry beyond the deterministic limit")
	}
}
