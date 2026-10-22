package sandboxlogs

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
)

func TestLogWindowPersistsAcrossStoreRestartAndRetainsNewestLines(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gateway.db")
	store, err := lifecycle.NewSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]Line, 2001)
	for i := range batch {
		batch[i] = Line{SandboxID: "sandbox-a", EventTime: time.Unix(int64(i+1), 0).UTC(), Level: "INFO", Source: "sandbox", Message: fmt.Sprintf("event-%04d", i)}
	}
	if _, err := service.Append(ctx, "sandbox-a", batch); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := lifecycle.NewSQLiteStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := New(reopened)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := restored.List(ctx, "sandbox-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != maxLines || lines[0].Message != "event-0001" || lines[0].Sequence != 2 || lines[len(lines)-1].Message != "event-2000" || lines[len(lines)-1].Sequence != 2001 {
		t.Fatalf("retained window count=%d first=%q last=%q", len(lines), lines[0].Message, lines[len(lines)-1].Message)
	}
}

func TestAppendRejectsMismatchedSandboxAndOversizedMessage(t *testing.T) {
	service, err := New(lifecycle.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Append(context.Background(), "sandbox-a", []Line{{SandboxID: "sandbox-b", Message: "event"}}); err == nil {
		t.Fatal("accepted a line bound to a different sandbox")
	}
	if _, err := service.Append(context.Background(), "sandbox-a", []Line{{SandboxID: "sandbox-a", Message: string(make([]byte, 64*1024+1))}}); err == nil {
		t.Fatal("accepted an oversized message")
	}
}
