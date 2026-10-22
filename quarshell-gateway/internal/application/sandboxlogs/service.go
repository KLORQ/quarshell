// Package sandboxlogs persists bounded supervisor log windows per sandbox.
package sandboxlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
)

const (
	recordKind = "sandbox_log_window"
	maxLines   = 2000
	maxBytes   = 8 << 20
)

// Line is one sandbox-correlated Gateway or supervisor log observation.
type Line struct {
	SandboxID string    `json:"sandbox_id"`
	Sequence  uint64    `json:"sequence,omitempty"`
	EventTime time.Time `json:"event_time,omitempty"`
	Level     string    `json:"level,omitempty"`
	Target    string    `json:"target,omitempty"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// Service owns each sandbox's durable bounded log window.
type Service struct{ store lifecycle.Store }

type window struct {
	NextSequence uint64 `json:"next_sequence"`
	Lines        []Line `json:"lines"`
}

// New returns a log service backed by the shared lifecycle store.
func New(store lifecycle.Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("sandbox logs: lifecycle store is required")
	}
	return &Service{store: store}, nil
}

// Append adds one batch atomically, retaining at most 2000 lines and 8 MiB.
func (s *Service) Append(ctx context.Context, sandboxID string, batch []Line) ([]Line, error) {
	sandboxID = strings.TrimSpace(sandboxID)
	if sandboxID == "" || len(sandboxID) > 256 {
		return nil, fmt.Errorf("sandbox logs: invalid sandbox ID")
	}
	for _, line := range batch {
		if line.SandboxID != sandboxID || strings.TrimSpace(line.Message) == "" || len(line.Message) > 64*1024 {
			return nil, fmt.Errorf("sandbox logs: invalid line identity or message")
		}
	}
	if len(batch) == 0 {
		return nil, nil
	}
	var appended []Line
	err := s.store.Update(ctx, func(tx lifecycle.WriteTx) error {
		record, exists, err := tx.Get(recordKind, sandboxID)
		if err != nil {
			return err
		}
		state := window{Lines: make([]Line, 0, len(batch))}
		if exists {
			if err := json.Unmarshal(record.Payload, &state); err != nil {
				// Read windows written by the first SQLite-backed log revision.
				if legacyErr := json.Unmarshal(record.Payload, &state.Lines); legacyErr != nil {
					return fmt.Errorf("sandbox logs: decode persisted window: %w", err)
				}
				for index := range state.Lines {
					state.NextSequence++
					state.Lines[index].Sequence = state.NextSequence
				}
			}
		}
		for _, line := range batch {
			if state.NextSequence == ^uint64(0) {
				return fmt.Errorf("sandbox logs: sequence exhausted")
			}
			state.NextSequence++
			line.Sequence = state.NextSequence
			state.Lines = append(state.Lines, line)
			appended = append(appended, line)
		}
		if len(state.Lines) > maxLines {
			state.Lines = state.Lines[len(state.Lines)-maxLines:]
		}
		if encodedSize(state.Lines) > maxBytes {
			low, high := 1, len(state.Lines)
			for low < high {
				mid := low + (high-low)/2
				if encodedSize(state.Lines[mid:]) <= maxBytes {
					high = mid
				} else {
					low = mid + 1
				}
			}
			state.Lines = state.Lines[low:]
		}
		payload, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("sandbox logs: encode window: %w", err)
		}
		version := uint64(0)
		if exists {
			version = record.Version
		}
		_, err = tx.Put(recordKind, sandboxID, version, payload)
		return err
	})
	return appended, err
}

// List returns a copy of the retained window for one sandbox.
func (s *Service) List(ctx context.Context, sandboxID string) ([]Line, error) {
	sandboxID = strings.TrimSpace(sandboxID)
	if sandboxID == "" || len(sandboxID) > 256 {
		return nil, fmt.Errorf("sandbox logs: invalid sandbox ID")
	}
	var state window
	err := s.store.View(ctx, func(tx lifecycle.ReadTx) error {
		record, exists, err := tx.Get(recordKind, sandboxID)
		if err != nil || !exists {
			return err
		}
		if err := json.Unmarshal(record.Payload, &state); err == nil && state.Lines != nil {
			return nil
		}
		if err := json.Unmarshal(record.Payload, &state.Lines); err != nil {
			return err
		}
		for index := range state.Lines {
			state.Lines[index].Sequence = uint64(index + 1)
		}
		return nil
	})
	return append([]Line(nil), state.Lines...), err
}

func encodedSize(lines []Line) int {
	payload, _ := json.Marshal(lines)
	return len(payload)
}
