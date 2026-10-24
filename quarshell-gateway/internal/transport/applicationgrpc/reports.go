package applicationgrpc

import (
	"context"

	"github.com/KLORQ/quarshell-core/domain"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReportProviderReadiness applies one supervisor readiness observation.
func (s *Server) ReportProviderReadiness(ctx context.Context, request *gatewayv1.ReportProviderReadinessRequest) (*gatewayv1.ReportProviderReadinessResponse, error) {
	if s.Sandboxes == nil || s.ProviderBindings == nil || s.SupervisorHub == nil {
		return nil, status.Error(codes.Unavailable, "provider readiness runtime is not configured")
	}
	ref, err := s.sandboxByID(ctx, request.GetSandboxId())
	if err != nil {
		return nil, err
	}
	observation := request.GetObservation()
	if observation == nil || observation.GetSequence() == 0 || observation.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider readiness observation, session_id and positive sequence are required")
	}
	if _, ok := gatewayv1.ProviderReadinessReason_name[int32(observation.GetReason())]; !ok {
		return nil, status.Error(codes.InvalidArgument, "provider readiness reason is invalid")
	}
	if s.readinessSession(string(ref.Sandbox)) != observation.GetSessionId() {
		return nil, status.Error(codes.FailedPrecondition, "provider readiness report is not from the active supervisor session")
	}
	instanceID, connectionEpoch, connected := s.SupervisorHub.SupervisorSession(string(ref.Sandbox))
	if !connected {
		return nil, status.Error(codes.FailedPrecondition, "sandbox supervisor relay is not connected")
	}
	principal, _ := sandboxPrincipalFromContext(ctx)
	if err := s.Sandboxes.ValidateSupervisorSession(ctx, ref, principal.generation, principal.credentialEpoch, instanceID, connectionEpoch); err != nil {
		return nil, status.Error(codes.Aborted, "sandbox supervisor session ownership changed")
	}
	bindings, err := s.ProviderBindings.List(ctx, ref)
	if err != nil {
		return nil, grpcError(err)
	}
	ready := observation.GetCredentialsInstalled() && observation.GetPolicyActive() && observation.GetLaunchEnvironmentInstalled()
	for _, binding := range bindings {
		if ready {
			_, err = s.ProviderBindings.Ack(ctx, ref, binding.Provider, binding.Revision)
		} else if readinessFailure(observation.GetReason()) {
			_, err = s.ProviderBindings.Fail(ctx, ref, binding.Provider, binding.Revision, observation.GetReason().String())
		} else {
			// Startup observations commonly report that credentials, policy or
			// process setup is still pending. Do not turn that transient state into
			// a terminal failure; only explicit installation failures do that.
			continue
		}
		if err != nil {
			return nil, grpcError(err)
		}
	}
	return &gatewayv1.ReportProviderReadinessResponse{AcceptedSequence: observation.GetSequence()}, nil
}

func readinessFailure(reason gatewayv1.ProviderReadinessReason) bool {
	switch reason {
	case gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_CREDENTIAL_INSTALL_FAILED,
		gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_POLICY_ACTIVATION_FAILED,
		gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_PROCESS_INSTALL_FAILED:
		return true
	default:
		return false
	}
}

// ReportEndpointStatus accepts an endpoint observation batch.
func (s *Server) ReportEndpointStatus(ctx context.Context, request *gatewayv1.ReportEndpointStatusRequest) (*gatewayv1.ReportEndpointStatusResponse, error) {
	if s.Sandboxes == nil {
		return nil, status.Error(codes.Unavailable, "endpoint status runtime is not configured")
	}
	if _, err := s.sandboxByID(ctx, request.GetSandboxId()); err != nil {
		return nil, err
	}
	return &gatewayv1.ReportEndpointStatusResponse{}, nil
}

// ReportSandboxConfiguration accepts a sandbox configuration admission report.
func (s *Server) ReportSandboxConfiguration(ctx context.Context, request *gatewayv1.ReportSandboxConfigurationRequest) (*gatewayv1.ReportSandboxConfigurationResponse, error) {
	if s.Sandboxes == nil {
		return nil, status.Error(codes.Unavailable, "sandbox configuration runtime is not configured")
	}
	if _, err := s.sandboxByID(ctx, request.GetSandboxId()); err != nil {
		return nil, err
	}
	return &gatewayv1.ReportSandboxConfigurationResponse{}, nil
}

// GetSandboxProviderEnvironment reports the bound provider environment revision.
func (s *Server) GetSandboxProviderEnvironment(ctx context.Context, request *gatewayv1.GetSandboxProviderEnvironmentRequest) (*gatewayv1.GetSandboxProviderEnvironmentResponse, error) {
	if s.Sandboxes == nil || s.ProviderBindings == nil {
		return nil, status.Error(codes.Unavailable, "provider environment runtime is not configured")
	}
	ref, err := s.sandboxByID(ctx, request.GetSandboxId())
	if err != nil {
		return nil, err
	}
	bindings, err := s.ProviderBindings.List(ctx, ref)
	if err != nil {
		return nil, grpcError(err)
	}
	environment := make(map[string]string)
	var revision uint64
	withheldCredentials := false
	if s.Credentials == nil {
		return nil, status.Error(codes.Unavailable, "credential store is not configured")
	}
	for _, binding := range bindings {
		if binding.Revision > revision {
			revision = binding.Revision
		}
		secrets, err := s.Credentials.List(ctx, ref.Workspace, binding.Provider)
		if err != nil {
			return nil, grpcError(err)
		}
		for _, secret := range secrets {
			if secret.ExpiresAt.IsZero() || s.now().Before(secret.ExpiresAt) {
				// Provider credential records do not yet carry validated endpoint
				// bindings. Returning a secret as ordinary environment would make it
				// usable at every destination, so keep it withheld until the policy
				// and supervisor binding contract is wired end to end.
				withheldCredentials = true
			}
		}
	}
	readiness := gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_UNSPECIFIED
	if withheldCredentials {
		readiness = gatewayv1.ProviderReadinessReason_PROVIDER_READINESS_REASON_CREDENTIALS_WITHHELD
	}
	return &gatewayv1.GetSandboxProviderEnvironmentResponse{
		Environment: environment, ProviderEnvRevision: revision, ReadinessReason: readiness,
	}, nil
}

func (s *Server) sandboxByID(ctx context.Context, id string) (domain.SandboxRef, error) {
	if id == "" {
		return domain.SandboxRef{}, status.Error(codes.InvalidArgument, "sandbox_id is required")
	}
	principal, ok := sandboxPrincipalFromContext(ctx)
	if !ok {
		return domain.SandboxRef{}, status.Error(codes.Unauthenticated, "sandbox principal required")
	}
	if string(principal.sandbox) != id {
		return domain.SandboxRef{}, status.Error(codes.PermissionDenied, "sandbox does not match authenticated principal")
	}
	current, err := s.Sandboxes.GetBySandboxID(ctx, principal.sandbox)
	if err != nil {
		return domain.SandboxRef{}, grpcError(err)
	}
	if current.State != "running" || current.RuntimeGenerationOrInitial() != principal.generation || current.CredentialEpochOrInitial() != principal.credentialEpoch {
		return domain.SandboxRef{}, status.Error(codes.FailedPrecondition, "sandbox credential does not match the active runtime generation")
	}
	return domain.SandboxRef{Workspace: current.Workspace, Sandbox: current.Sandbox}, nil
}
