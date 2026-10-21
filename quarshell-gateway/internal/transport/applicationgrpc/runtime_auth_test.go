package applicationgrpc

import (
	"context"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	sandboxapp "github.com/KLORQ/quarshell-gateway/internal/application/sandbox"
	tokenapp "github.com/KLORQ/quarshell-gateway/internal/application/token"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRefreshSandboxTokenRequiresCurrentGeneration(t *testing.T) {
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ref := domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}
	ctx := context.Background()
	if _, _, err := sandboxes.Create(ctx, sandboxapp.Request{Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "image"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	started, _, err := sandboxes.Start(ctx, ref, 1, "start-1")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	tokens, err := tokenapp.NewService([]byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewService token: %v", err)
	}
	raw, _, err := tokens.IssueForAudienceEpoch(ref, started.RuntimeGeneration, 2, "gateway", "supervisor", time.Minute)
	if err != nil {
		t.Fatalf("IssueForGeneration: %v", err)
	}
	incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+raw))
	principalCtx, err := withSandboxPrincipal(incoming, tokens)
	if err != nil {
		t.Fatalf("withSandboxPrincipal: %v", err)
	}
	server := New("test", nil, sandboxes, nil, nil, nil, nil, nil)
	server.Tokens = tokens
	response, err := server.RefreshSandboxToken(principalCtx, &gatewayv1.RefreshSandboxTokenRequest{})
	if err != nil {
		t.Fatalf("RefreshSandboxToken: %v", err)
	}
	refreshed, err := tokens.Verify(response.GetToken())
	if err != nil || refreshed.Generation != started.RuntimeGeneration || response.GetSessionId() != "g0000000000000002" {
		t.Fatalf("refreshed claims=%+v session_id=%q err=%v", refreshed, response.GetSessionId(), err)
	}
	if _, err := sandboxes.Observe(ctx, ref, 2, "running"); err != nil {
		t.Fatalf("advance sandbox generation: %v", err)
	}
	if _, err := server.RefreshSandboxToken(principalCtx, &gatewayv1.RefreshSandboxTokenRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("stale refresh status=%s err=%v, want Unauthenticated", status.Code(err), err)
	}
}

func TestSandboxTokensAreRestrictedToSupervisorMethods(t *testing.T) {
	tokens, err := tokenapp.NewService([]byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewService token: %v", err)
	}
	raw, _, err := tokens.IssueForGeneration(context.Background(), domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}, 1, "supervisor", time.Minute)
	if err != nil {
		t.Fatalf("IssueForGeneration: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+raw))
	unary := UnaryAuthInterceptorWithSandboxTokens("operator", "gateway", tokens)
	if _, err := unary(ctx, &gatewayv1.GetCurrentUserRequest{}, &grpc.UnaryServerInfo{FullMethod: gatewayv1.OpenShell_GetCurrentUser_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		return nil, status.Error(codes.Internal, "handler should not run")
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sandbox bearer on operator RPC status=%s err=%v", status.Code(err), err)
	}
	if _, err := unary(ctx, &gatewayv1.RefreshSandboxTokenRequest{}, &grpc.UnaryServerInfo{FullMethod: gatewayv1.OpenShell_RefreshSandboxToken_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		if _, ok := sandboxPrincipalFromContext(ctx); !ok {
			return nil, status.Error(codes.Internal, "sandbox principal missing")
		}
		return &gatewayv1.RefreshSandboxTokenResponse{}, nil
	}); err != nil {
		t.Fatalf("sandbox bearer on refresh RPC: %v", err)
	}

	streamAuth := StreamAuthInterceptorWithSandboxTokens("operator", "gateway", tokens)
	handler := func(_ any, stream grpc.ServerStream) error {
		if _, ok := sandboxPrincipalFromContext(stream.Context()); !ok {
			return status.Error(codes.Internal, "sandbox principal missing")
		}
		return nil
	}
	if err := streamAuth(nil, fixtureServerStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: gatewayv1.OpenShell_ConnectSupervisor_FullMethodName}, handler); err != nil {
		t.Fatalf("sandbox bearer on ConnectSupervisor: %v", err)
	}
	if err := streamAuth(nil, fixtureServerStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: gatewayv1.OpenShell_PeerRelay_FullMethodName}, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sandbox bearer on PeerRelay status=%s err=%v", status.Code(err), err)
	}
	sandboxToken, _, err := tokens.IssueForAudience(domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}, 1, "sandbox", "supervisor", time.Minute)
	if err != nil {
		t.Fatalf("IssueForAudience: %v", err)
	}
	sandboxCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+sandboxToken))
	if _, err := unary(sandboxCtx, &gatewayv1.RefreshSandboxTokenRequest{}, &grpc.UnaryServerInfo{FullMethod: gatewayv1.OpenShell_RefreshSandboxToken_FullMethodName}, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.Internal, "handler should not run")
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sandbox-protocol bearer on Gateway refresh status=%s err=%v", status.Code(err), err)
	}
}
