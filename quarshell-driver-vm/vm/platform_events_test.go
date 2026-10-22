//go:build darwin || linux

package vm

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	core "github.com/KLORQ/quarshell-core"
	"github.com/KLORQ/quarshell-driver/driver"
)

func TestVMPlatformWatchSnapshotsLifecycleAndDeletion(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	d := NewWithHostConfig(HostConfig{StateDir: stateDir})
	store, err := openStateStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.create("watched", "image:latest", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := d.WatchPlatformEvents(ctx, driver.PlatformEventWatchRequest{SandboxID: core.ID(state.ID)})
	if err != nil {
		t.Fatal(err)
	}
	first := receiveVMPlatformEvent(t, stream)
	if first.Event.SandboxID != core.ID(state.ID) || first.Event.Reason != "created" || first.Event.Metadata["status"] != "created" {
		t.Fatalf("initial event = %+v", first)
	}
	state.Status = "running"
	state.UpdatedAt = time.Now().UTC().Add(time.Second)
	if err := store.save(state); err != nil {
		t.Fatal(err)
	}
	changed := receiveVMPlatformEvent(t, stream)
	if changed.Event.Reason != "running" || changed.Event.Terminal {
		t.Fatalf("state event = %+v", changed)
	}
	if err := store.delete(state.ID); err != nil {
		t.Fatal(err)
	}
	deleted := receiveVMPlatformEvent(t, stream)
	if deleted.Event.Type != "deleted" || !deleted.Event.Terminal {
		t.Fatalf("delete event = %+v", deleted)
	}
	if _, open := <-stream; open {
		t.Fatal("stream remained open after deletion")
	}
}

func receiveVMPlatformEvent(t *testing.T, stream <-chan driver.PlatformEventResult) driver.PlatformEventResult {
	t.Helper()
	select {
	case event, open := <-stream:
		if !open {
			t.Fatal("platform event stream closed")
		}
		if event.Err != nil {
			t.Fatalf("platform event error: %v", event.Err)
		}
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for VM platform event")
		return driver.PlatformEventResult{}
	}
}
