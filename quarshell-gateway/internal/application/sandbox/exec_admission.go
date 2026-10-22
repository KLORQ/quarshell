package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
)

const execAdmissionRetention = 24 * time.Hour

type execAdmissionState struct {
	Completed bool `json:"completed"`
}

// AdmitExecRequest durably fences a launch before it reaches a runtime. An
// unfinished record is retained indefinitely because a disconnected caller
// cannot know whether the guest started the command. A completed request ID
// may be reused after the upstream 24-hour retention window.
func (s *Service) AdmitExecRequest(ctx context.Context, ref domain.SandboxRef, requestID, requestHash string) error {
	if ctx == nil || s == nil || s.store == nil {
		return domain.Invalidf("sandbox.exec_admission", "context and lifecycle store are required")
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || requestHash == "" || ref.Workspace == "" || ref.Sandbox == "" {
		return domain.Invalidf("sandbox.exec_admission", "workspace, sandbox, request ID, and request hash are required")
	}
	key := execAdmissionKey(ref, requestID)
	now := s.now().UTC()
	return s.store.Update(ctx, func(tx lifecycle.WriteTx) error {
		previous, exists, err := tx.GetOperation(key)
		if err != nil {
			return err
		}
		if exists {
			var state execAdmissionState
			_ = json.Unmarshal(previous.Result, &state)
			if !state.Completed || now.Sub(previous.CreatedAt) < execAdmissionRetention {
				return domain.New(domain.CodeConflict, "sandbox.exec_admission", "request_id has already been admitted")
			}
			if err := tx.DeleteOperation(key); err != nil {
				return err
			}
		}
		result, err := json.Marshal(execAdmissionState{})
		if err != nil {
			return fmt.Errorf("encode exec admission: %w", err)
		}
		return tx.PutOperation(lifecycle.Operation{Key: key, RequestHash: requestHash, Result: result, CreatedAt: now})
	})
}

// CompleteExecRequest starts the 24-hour duplicate-retention window after the
// command reaches a known terminal result.
func (s *Service) CompleteExecRequest(ctx context.Context, ref domain.SandboxRef, requestID string) error {
	if ctx == nil || s == nil || s.store == nil {
		return domain.Invalidf("sandbox.exec_admission", "context and lifecycle store are required")
	}
	key := execAdmissionKey(ref, strings.TrimSpace(requestID))
	now := s.now().UTC()
	return s.store.Update(ctx, func(tx lifecycle.WriteTx) error {
		operation, exists, err := tx.GetOperation(key)
		if err != nil {
			return err
		}
		if !exists {
			return domain.New(domain.CodeConflict, "sandbox.exec_admission", "request_id was not admitted")
		}
		result, err := json.Marshal(execAdmissionState{Completed: true})
		if err != nil {
			return fmt.Errorf("encode completed exec admission: %w", err)
		}
		if err := tx.DeleteOperation(key); err != nil {
			return err
		}
		operation.Result = result
		operation.CreatedAt = now
		return tx.PutOperation(operation)
	})
}

func execAdmissionKey(ref domain.SandboxRef, requestID string) string {
	return "sandbox.exec_admission:" + string(ref.Workspace) + "/" + string(ref.Sandbox) + ":" + requestID
}
