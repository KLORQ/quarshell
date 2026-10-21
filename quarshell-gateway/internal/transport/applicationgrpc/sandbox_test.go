package applicationgrpc

import (
	"context"
	"testing"

	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	"github.com/KLORQ/quarshell-gateway/internal/application/sandbox"
	datamodelv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
)

func TestSandboxRPC(t *testing.T) {
	sandboxes, err := sandbox.NewService(lifecycle.NewMemoryStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	server := New("test", nil, sandboxes, nil, nil, nil, nil, nil)
	ctx := context.Background()
	selector := &datamodelv1.WorkspaceSelector{Selection: &datamodelv1.WorkspaceSelector_Workspace{Workspace: "ws"}}
	created, err := server.CreateSandbox(ctx, &gatewayv1.CreateSandboxRequest{
		WorkspaceScope: selector, Name: "sb", Labels: map[string]string{"team": "x"},
		Spec: &gatewayv1.SandboxSpec{Template: &gatewayv1.SandboxTemplate{Image: "ubuntu:24.04"}},
	})
	if err != nil || created.GetSandbox().GetMetadata().GetName() != "sb" {
		t.Fatalf("CreateSandbox = %+v err=%v", created, err)
	}
	if _, err := server.GetSandbox(ctx, &gatewayv1.GetSandboxRequest{WorkspaceScope: selector, Name: "sb"}); err != nil {
		t.Fatalf("GetSandbox: %v", err)
	}
	list, err := server.ListSandboxes(ctx, &gatewayv1.ListSandboxesRequest{WorkspaceScope: selector})
	if err != nil || len(list.GetSandboxes()) != 1 {
		t.Fatalf("ListSandboxes = %+v err=%v", list, err)
	}
	started, err := server.StartSandbox(ctx, &gatewayv1.StartSandboxRequest{WorkspaceScope: selector, Name: "sb"})
	if err != nil || started.GetSandbox().GetStatus().GetPhase() != gatewayv1.SandboxPhase_SANDBOX_PHASE_READY {
		t.Fatalf("StartSandbox = %+v err=%v", started, err)
	}
	stopped, err := server.StopSandbox(ctx, &gatewayv1.StopSandboxRequest{WorkspaceScope: selector, Name: "sb"})
	if err != nil || stopped.GetSandbox().GetStatus().GetPhase() != gatewayv1.SandboxPhase_SANDBOX_PHASE_STOPPED {
		t.Fatalf("StopSandbox = %+v err=%v", stopped, err)
	}
	deleted, err := server.DeleteSandbox(ctx, &gatewayv1.DeleteSandboxRequest{WorkspaceScope: selector, Name: "sb"})
	if err != nil || deleted.GetOutcome() != gatewayv1.DeletionOutcome_DELETION_OUTCOME_COMPLETED {
		t.Fatalf("DeleteSandbox = %+v err=%v", deleted, err)
	}
	missing, err := server.DeleteSandbox(ctx, &gatewayv1.DeleteSandboxRequest{WorkspaceScope: selector, Name: "sb", AllowMissing: true})
	if err != nil || missing.GetOutcome() != gatewayv1.DeletionOutcome_DELETION_OUTCOME_ALREADY_ABSENT {
		t.Fatalf("Delete missing = %+v err=%v", missing, err)
	}
}
