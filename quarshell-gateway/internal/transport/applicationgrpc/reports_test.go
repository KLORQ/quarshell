package applicationgrpc

import (
	"context"
	"testing"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	"github.com/KLORQ/quarshell-gateway/internal/application/provider"
	"github.com/KLORQ/quarshell-gateway/internal/application/providercred"
	sandboxapp "github.com/KLORQ/quarshell-gateway/internal/application/sandbox"
	"github.com/KLORQ/quarshell-gateway/internal/sshrelay"
	"github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestReportProviderReadinessRequiresActiveSessionAndKeepsPendingStartup(t *testing.T) {
	ctx := context.Background()
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.SandboxRef{Workspace: "workspace", Sandbox: "sandbox-ready"}
	if _, _, err := sandboxes.Create(ctx, sandboxapp.Request{Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "image"}); err != nil {
		t.Fatal(err)
	}
	started, _, err := sandboxes.Start(ctx, ref, 1, "start-readiness")
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := provider.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := bindings.Bind(ctx, provider.BindRequest{Sandbox: ref, Provider: "provider-a", Revision: 3}); err != nil {
		t.Fatal(err)
	}
	instanceID, connectionEpoch, sessionID := "supervisor-1", uint64(1), "session-current"
	if err := sandboxes.ClaimSupervisorSession(ctx, ref, started.RuntimeGenerationOrInitial(), started.CredentialEpochOrInitial(), sessionID, instanceID, connectionEpoch); err != nil {
		t.Fatal(err)
	}
	hub := sshrelay.NewHub()
	_, err = hub.RegisterSupervisor(string(ref.Sandbox), instanceID, connectionEpoch, func(string, string) error { return nil }, make(chan struct{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	server := New("test", nil, sandboxes, nil, nil, nil, nil, nil)
	server.ProviderBindings = bindings
	server.SupervisorHub = hub
	server.setReadinessSession(string(ref.Sandbox), sessionID)
	sandboxCtx := context.WithValue(ctx, principalContextKey{}, authenticatedPrincipal{
		identityProvider: "sandbox-token", sandbox: ref.Sandbox,
		generation: started.RuntimeGenerationOrInitial(), credentialEpoch: started.CredentialEpochOrInitial(),
	})
	request := &gatewayv1.ReportProviderReadinessRequest{
		SandboxId: string(ref.Sandbox),
		Observation: &gatewayv1.ProviderReadinessObservation{
			SessionId: sessionID, Sequence: 1,
			Reason: gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_WAITING_FOR_CREDENTIALS,
		},
	}
	if _, err := server.ReportProviderReadiness(sandboxCtx, request); err != nil {
		t.Fatalf("pending startup report: %v", err)
	}
	binding, err := bindings.Get(ctx, ref, "provider-a")
	if err != nil || binding.Readiness != provider.ReadinessPending {
		t.Fatalf("incomplete report changed readiness to %q, err=%v", binding.Readiness, err)
	}

	wrongSession := proto.Clone(request).(*gatewayv1.ReportProviderReadinessRequest)
	wrongSession.GetObservation().SessionId = "stale-session"
	if _, err := server.ReportProviderReadiness(sandboxCtx, wrongSession); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("stale session status = %v err=%v", status.Code(err), err)
	}
	missingSequence := proto.Clone(request).(*gatewayv1.ReportProviderReadinessRequest)
	missingSequence.GetObservation().Sequence = 0
	if _, err := server.ReportProviderReadiness(sandboxCtx, missingSequence); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing sequence status = %v err=%v", status.Code(err), err)
	}

	ready := proto.Clone(request).(*gatewayv1.ReportProviderReadinessRequest)
	readyObservation := ready.GetObservation()
	readyObservation.Sequence = 2
	readyObservation.Reason = gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_UNSPECIFIED
	readyObservation.CredentialsInstalled = true
	readyObservation.PolicyActive = true
	readyObservation.LaunchEnvironmentInstalled = true
	if _, err := server.ReportProviderReadiness(sandboxCtx, ready); err != nil {
		t.Fatalf("ready report: %v", err)
	}
	binding, err = bindings.Get(ctx, ref, "provider-a")
	if err != nil || binding.Readiness != provider.ReadinessReady {
		t.Fatalf("complete report readiness = %q, err=%v", binding.Readiness, err)
	}
}

func TestSandboxRuntimeReportScopeAndGeneration(t *testing.T) {
	service, err := sandboxapp.NewService(lifecycle.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.SandboxID{"sandbox-a", "sandbox-b"} {
		ref := domain.SandboxRef{Workspace: "workspace", Sandbox: id}
		if _, _, err := service.Create(context.Background(), sandboxapp.Request{Workspace: ref.Workspace, Sandbox: id, Generation: 1, Image: "image"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.Start(context.Background(), ref, 1, "start-"+domain.OperationID(id)); err != nil {
			t.Fatal(err)
		}
	}
	server := New("test", nil, service, nil, nil, nil, nil, nil)
	ctx := context.WithValue(context.Background(), principalContextKey{}, authenticatedPrincipal{
		identityProvider: "sandbox-token", sandbox: "sandbox-a", generation: 2, credentialEpoch: 2,
	})
	if _, err := server.sandboxByID(ctx, "sandbox-b"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-sandbox lookup status=%s err=%v", status.Code(err), err)
	}
	stale := context.WithValue(context.Background(), principalContextKey{}, authenticatedPrincipal{
		identityProvider: "sandbox-token", sandbox: "sandbox-a", generation: 3, credentialEpoch: 2,
	})
	if _, err := server.sandboxByID(stale, "sandbox-a"); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("stale generation status=%s err=%v", status.Code(err), err)
	}
	if _, err := server.sandboxByID(ctx, "sandbox-a"); err != nil {
		t.Fatalf("current runtime principal: %v", err)
	}
}

func TestGetSandboxProviderEnvironmentWithholdsUnboundSecrets(t *testing.T) {
	ctx := context.Background()
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.SandboxRef{Workspace: "workspace", Sandbox: "sandbox-a"}
	if _, _, err := sandboxes.Create(ctx, sandboxapp.Request{Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "image"}); err != nil {
		t.Fatal(err)
	}
	started, _, err := sandboxes.Start(ctx, ref, 1, "start-sandbox-a")
	if err != nil {
		t.Fatal(err)
	}
	providers, err := provider.NewRegistry(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := providers.Create(ctx, provider.CreateRequest{ID: "p", Name: "p", Driver: "api"}); err != nil {
		t.Fatal(err)
	}
	bindings, err := provider.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := bindings.Bind(ctx, provider.BindRequest{Sandbox: ref, Provider: "p", Revision: 1}); err != nil {
		t.Fatal(err)
	}
	credentials, err := providercred.NewStore(store, []byte("test encryption key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := credentials.Put(ctx, providercred.Secret{Workspace: ref.Workspace, Provider: "p", Key: "ACCESS_TOKEN", Value: "secret-value"}); err != nil {
		t.Fatal(err)
	}
	server := New("test", nil, sandboxes, nil, nil, nil, providers, nil)
	server.ProviderBindings = bindings
	server.Credentials = credentials
	sandboxCtx := context.WithValue(ctx, principalContextKey{}, authenticatedPrincipal{
		identityProvider: "sandbox-token", sandbox: ref.Sandbox,
		generation: started.RuntimeGeneration, credentialEpoch: started.CredentialEpoch,
	})
	response, err := server.GetSandboxProviderEnvironment(sandboxCtx, &gatewayv1.GetSandboxProviderEnvironmentRequest{
		SandboxId: string(ref.Sandbox), SupportsStaticCredentialBindings: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := response.GetEnvironment()["ACCESS_TOKEN"]; leaked || response.GetReadinessReason() != gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_CREDENTIALS_WITHHELD {
		t.Fatalf("unbound provider credential was not withheld: %+v", response)
	}
}
