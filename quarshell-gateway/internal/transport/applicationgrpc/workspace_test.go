package applicationgrpc

import (
	"context"
	"testing"

	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	"github.com/KLORQ/quarshell-gateway/internal/application/workspace"
	datamodelv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var workspaceSelector = datamodelv1.WorkspaceSelector{Selection: &datamodelv1.WorkspaceSelector_Workspace{Workspace: "ws"}}

func TestWorkspaceRPC(t *testing.T) {
	workspaces, err := workspace.NewService(lifecycle.NewMemoryStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	server := New("test", workspaces, nil, nil, nil, nil, nil, nil)
	ctx := workspacePrincipalContext("gateway-admin", "platform_admin")
	created, err := server.CreateWorkspace(ctx, &gatewayv1.CreateWorkspaceRequest{Name: "ws", RequestId: "create-ws"})
	if err != nil || created.GetWorkspace().GetMetadata().GetName() != "ws" {
		t.Fatalf("CreateWorkspace = %+v err=%v", created, err)
	}
	got, err := server.GetWorkspace(ctx, &gatewayv1.GetWorkspaceRequest{Name: "ws"})
	if err != nil || got.GetWorkspace().GetMetadata().GetId() != "ws" {
		t.Fatalf("GetWorkspace = %+v err=%v", got, err)
	}
	list, err := server.ListWorkspaces(ctx, &gatewayv1.ListWorkspacesRequest{PageSize: 1})
	if err != nil || len(list.GetWorkspaces()) != 1 {
		t.Fatalf("ListWorkspaces = %+v err=%v", list, err)
	}
	added, err := server.AddWorkspaceMember(ctx, &gatewayv1.AddWorkspaceMemberRequest{
		WorkspaceScope: &workspaceSelector, PrincipalSubject: "alice", Role: gatewayv1.WorkspaceRole_WORKSPACE_ROLE_ADMIN,
	})
	if err != nil || added.GetMember().GetPrincipalSubject() != "alice" {
		t.Fatalf("AddWorkspaceMember = %+v err=%v", added, err)
	}
	members, err := server.ListWorkspaceMembers(ctx, &gatewayv1.ListWorkspaceMembersRequest{WorkspaceScope: &workspaceSelector})
	if err != nil || len(members.GetMembers()) != 2 {
		t.Fatalf("ListWorkspaceMembers = %+v err=%v", members, err)
	}
	if _, err := server.RemoveWorkspaceMember(ctx, &gatewayv1.RemoveWorkspaceMemberRequest{WorkspaceScope: &workspaceSelector, PrincipalSubject: "alice"}); err != nil {
		t.Fatalf("RemoveWorkspaceMember: %v", err)
	}
	deleted, err := server.DeleteWorkspace(ctx, &gatewayv1.DeleteWorkspaceRequest{Name: "ws"})
	if err != nil || deleted.GetOutcome() != gatewayv1.DeletionOutcome_DELETION_OUTCOME_COMPLETED {
		t.Fatalf("DeleteWorkspace = %+v err=%v", deleted, err)
	}
}

func TestWorkspaceRPCEnforcesPrincipalMembershipAndRole(t *testing.T) {
	workspaces, err := workspace.NewService(lifecycle.NewMemoryStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	server := New("test", workspaces, nil, nil, nil, nil, nil, nil)
	alice := workspacePrincipalContext("alice")
	bob := workspacePrincipalContext("bob")
	admin := workspacePrincipalContext("root", "platform_admin")

	for _, name := range []string{"alice-ws", "bob-ws"} {
		ctx := alice
		if name == "bob-ws" {
			ctx = bob
		}
		if _, err := server.CreateWorkspace(ctx, &gatewayv1.CreateWorkspaceRequest{Name: name, RequestId: "create-" + name}); err != nil {
			t.Fatalf("CreateWorkspace(%s): %v", name, err)
		}
	}
	if _, err := server.GetWorkspace(alice, &gatewayv1.GetWorkspaceRequest{Name: "bob-ws"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-workspace GetWorkspace status=%s, want PermissionDenied", status.Code(err))
	}
	if _, err := server.AddWorkspaceMember(alice, &gatewayv1.AddWorkspaceMemberRequest{
		WorkspaceScope:   &datamodelv1.WorkspaceSelector{Selection: &datamodelv1.WorkspaceSelector_Workspace{Workspace: "bob-ws"}},
		PrincipalSubject: "mallory", Role: gatewayv1.WorkspaceRole_WORKSPACE_ROLE_ADMIN,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-workspace AddWorkspaceMember status=%s, want PermissionDenied", status.Code(err))
	}
	listed, err := server.ListWorkspaces(alice, &gatewayv1.ListWorkspacesRequest{})
	if err != nil || len(listed.GetWorkspaces()) != 1 || listed.GetWorkspaces()[0].GetMetadata().GetName() != "alice-ws" {
		t.Fatalf("workspace list for alice=%+v err=%v, want only alice-ws", listed, err)
	}
	if _, err := server.DeleteWorkspace(alice, &gatewayv1.DeleteWorkspaceRequest{Name: "bob-ws"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-workspace DeleteWorkspace status=%s, want PermissionDenied", status.Code(err))
	}
	if _, err := server.GetWorkspace(context.Background(), &gatewayv1.GetWorkspaceRequest{Name: "alice-ws"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous GetWorkspace status=%s, want Unauthenticated", status.Code(err))
	}

	selector := &datamodelv1.WorkspaceSelector{Selection: &datamodelv1.WorkspaceSelector_Workspace{Workspace: "alice-ws"}}
	if _, err := server.AddWorkspaceMember(alice, &gatewayv1.AddWorkspaceMemberRequest{
		WorkspaceScope: selector, PrincipalSubject: "bob", Role: gatewayv1.WorkspaceRole_WORKSPACE_ROLE_USER, RequestId: "add-bob",
	}); err != nil {
		t.Fatalf("owner AddWorkspaceMember: %v", err)
	}
	if _, err := server.AddWorkspaceMember(bob, &gatewayv1.AddWorkspaceMemberRequest{
		WorkspaceScope: selector, PrincipalSubject: "mallory", Role: gatewayv1.WorkspaceRole_WORKSPACE_ROLE_USER,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("member AddWorkspaceMember status=%s, want PermissionDenied", status.Code(err))
	}
	if _, err := server.DeleteWorkspace(bob, &gatewayv1.DeleteWorkspaceRequest{Name: "alice-ws"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("member DeleteWorkspace status=%s, want PermissionDenied", status.Code(err))
	}
	if _, err := server.GetWorkspace(admin, &gatewayv1.GetWorkspaceRequest{Name: "bob-ws"}); err != nil {
		t.Fatalf("platform admin cross-workspace access: %v", err)
	}
}

func workspacePrincipalContext(subject string, roles ...string) context.Context {
	return context.WithValue(context.Background(), principalContextKey{}, authenticatedPrincipal{subject: subject, roles: roles})
}
