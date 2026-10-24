package applicationgrpc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	policyapp "github.com/KLORQ/quarshell-gateway/internal/application/policy"
	datamodelv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGlobalPolicyHistoryAndStatusSurviveSQLiteRestart(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "policies.db")
	store, err := lifecycle.NewSQLiteStore(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	policies, err := policyapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	global := policyapp.Scope{Workspace: "workspace-a"}
	first, _, err := policies.CreateDraft(ctx, policyapp.DraftRequest{Scope: global, Source: []byte("version: 1\n"), Author: "operator", IdempotencyKey: "global-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policies.Approve(ctx, global, first.ID, "operator", "approve-global-1"); err != nil {
		t.Fatal(err)
	}
	second, _, err := policies.CreateDraft(ctx, policyapp.DraftRequest{Scope: global, Source: []byte("version: 1\nfilesystem_policy:\n  read_only: [/usr]\n"), Author: "operator", IdempotencyKey: "global-2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policies.Approve(ctx, global, second.ID, "operator", "approve-global-2"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = lifecycle.NewSQLiteStore(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	policies, err = policyapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	server := New("test", nil, nil, nil, nil, nil, nil, nil)
	server.PolicyRevisions = policies
	selector := &datamodelv1.WorkspaceSelector{Selection: &datamodelv1.WorkspaceSelector_Workspace{Workspace: "workspace-a"}}
	page, err := server.ListSandboxPolicies(ctx, &gatewayv1.ListSandboxPoliciesRequest{WorkspaceScope: selector, Global: true, PageSize: 1})
	if err != nil || len(page.GetRevisions()) != 1 || page.GetRevisions()[0].GetVersion() != uint32(second.Number) || page.GetNextPageToken() == "" {
		t.Fatalf("global page one = %+v, %v", page, err)
	}
	page, err = server.ListSandboxPolicies(ctx, &gatewayv1.ListSandboxPoliciesRequest{WorkspaceScope: selector, Global: true, PageSize: 1, PageToken: page.GetNextPageToken()})
	if err != nil || len(page.GetRevisions()) != 1 || page.GetRevisions()[0].GetVersion() != uint32(first.Number) {
		t.Fatalf("global page two = %+v, %v", page, err)
	}
	statusResponse, err := server.GetSandboxPolicyStatus(ctx, &gatewayv1.GetSandboxPolicyStatusRequest{WorkspaceScope: selector, Global: true, Version: uint32(second.Number)})
	if err != nil || statusResponse.GetRevision().GetVersion() != uint32(second.Number) {
		t.Fatalf("global status = %+v, %v", statusResponse, err)
	}
	if _, err := server.GetSandboxPolicyStatus(ctx, &gatewayv1.GetSandboxPolicyStatusRequest{
		WorkspaceScope: selector, Global: true, Sandbox: "sandbox-a",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mixed global/sandbox scope = %v", err)
	}
	if _, err := server.ListSandboxPolicies(ctx, &gatewayv1.ListSandboxPoliciesRequest{Global: true}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("global list without workspace = %v", err)
	}
}
