package vm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	core "github.com/KLORQ/quarshell-core"
	"github.com/KLORQ/quarshell-driver/driver"
)

var _ driver.PlatformEventStream = (*Driver)(nil)

const vmPlatformWatchInterval = 100 * time.Millisecond

// WatchPlatformEvents emits the current VM state immediately, then reports
// persisted lifecycle changes and deletion. The VM state file is the source of
// truth, so reconnects always begin with a fresh snapshot.
func (d *Driver) WatchPlatformEvents(ctx context.Context, request driver.PlatformEventWatchRequest) (<-chan driver.PlatformEventResult, error) {
	if d == nil || ctx == nil {
		return nil, fmt.Errorf("vm platform watch: driver and context are required")
	}
	if request.SandboxID == "" {
		return nil, fmt.Errorf("vm platform watch: sandbox ID is required")
	}
	store, err := openStateStore(d.host.StateDir)
	if err != nil {
		return nil, err
	}
	state, err := store.load(string(request.SandboxID))
	if err != nil {
		return nil, err
	}
	results := make(chan driver.PlatformEventResult)
	go func() {
		defer close(results)
		last := state
		if !sendVMPlatformEvent(ctx, results, vmStateEvent(state, false)) {
			return
		}
		ticker := time.NewTicker(vmPlatformWatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				latest, loadErr := store.load(string(request.SandboxID))
				if loadErr != nil {
					if isMissingState(loadErr) {
						sendVMPlatformEvent(ctx, results, driver.PlatformEventResult{Event: driver.PlatformEvent{SandboxID: request.SandboxID, Time: time.Now().UTC(), Source: "vm", Type: "deleted", Reason: "sandbox_deleted", Terminal: true}})
						return
					}
					sendVMPlatformEvent(ctx, results, driver.PlatformEventResult{Err: fmt.Errorf("vm platform watch state: %w", loadErr)})
					return
				}
				if latest.Status == last.Status && latest.UpdatedAt.Equal(last.UpdatedAt) {
					continue
				}
				if !sendVMPlatformEvent(ctx, results, vmStateEvent(latest, latest.Status == "stopped" || latest.Status == "failed")) {
					return
				}
				last = latest
			}
		}
	}()
	return results, nil
}

func vmStateEvent(state sandboxState, terminal bool) driver.PlatformEventResult {
	return driver.PlatformEventResult{Event: driver.PlatformEvent{
		SandboxID: core.ID(state.ID), Time: state.UpdatedAt.UTC(), Source: "vm", Type: "sandbox_state",
		Reason: state.Status, Message: "VM sandbox state changed", Metadata: map[string]string{"status": state.Status}, Terminal: terminal,
	}}
}

func sendVMPlatformEvent(ctx context.Context, results chan<- driver.PlatformEventResult, event driver.PlatformEventResult) bool {
	select {
	case results <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func isMissingState(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
