package domain

import (
	"errors"
	"testing"
)

func TestFieldDistinguishesAbsentNullValue(t *testing.T) {
	var absent Field[int]
	if !absent.IsAbsent() || absent.IsNull() || absent.HasValue() {
		t.Fatalf("zero field must be absent")
	}
	if got := absent.OrElse(7); got != 7 {
		t.Fatalf("absent OrElse = %d, want 7", got)
	}
	null := Null[int]()
	if null.IsAbsent() || !null.IsNull() || null.HasValue() {
		t.Fatalf("null field classified incorrectly")
	}
	if got := null.OrElse(7); got != 7 {
		t.Fatalf("null OrElse = %d, want 7", got)
	}
	value := Some(0)
	if !value.HasValue() || value.Value != 0 {
		t.Fatalf("zero value must be present")
	}
	if err := null.RejectNull("parse", "process"); err == nil {
		t.Fatalf("explicit null must be rejected")
	}
	if err := absent.RejectNull("parse", "process"); err != nil {
		t.Fatalf("absent must not be rejected as null: %v", err)
	}
}

func TestGenerationExhaustion(t *testing.T) {
	next, err := Generation(1).Next()
	if err != nil || next != 2 {
		t.Fatalf("Next = %d, %v", next, err)
	}
	if _, err := Generation(^uint64(0)).Next(); !errors.Is(err, ErrConflict) {
		t.Fatalf("exhaustion error = %v, want conflict", err)
	}
}

func TestErrorClassification(t *testing.T) {
	wrapped := Wrap(ErrUnsupported, "compute.ensure", "backend capability unavailable")
	if !errors.Is(wrapped, ErrUnsupported) {
		t.Fatalf("wrapped error must match sentinel")
	}
	if CodeOf(wrapped) != CodeUnsupported {
		t.Fatalf("code = %s", CodeOf(wrapped))
	}
	if got := SafeMessage(wrapped); got != "backend capability unavailable" {
		t.Fatalf("safe message = %q", got)
	}
	if CodeOf(errors.New("boom")) != CodeInternal {
		t.Fatalf("unknown error must classify as internal")
	}
}

func TestCapabilitySet(t *testing.T) {
	set := NewCapabilitySet(CapExec, CapPTY)
	if !set.Has(CapExec) || set.Has(CapGPU) {
		t.Fatalf("unexpected capability membership")
	}
	if err := set.Require(CapPTY, "exec"); err != nil {
		t.Fatalf("require present: %v", err)
	}
	if err := set.Require(CapGPU, "exec"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("require absent = %v", err)
	}
}
