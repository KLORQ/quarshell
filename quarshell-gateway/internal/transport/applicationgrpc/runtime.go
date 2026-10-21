package applicationgrpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func unavailable(feature string) error {
	return status.Error(codes.Unimplemented, feature+" is not configured")
}

func credentialEpoch(epoch uint64) uint64 {
	if epoch == 0 {
		return 1
	}
	return epoch
}

// IssueSandboxToken requires the auth-bound sandbox principal.
func (s *Server) IssueSandboxToken(context.Context, *gatewayv1.IssueSandboxTokenRequest) (*gatewayv1.IssueSandboxTokenResponse, error) {
	return nil, unavailable("sandbox token issuance")
}

// RefreshSandboxToken requires the auth-bound sandbox principal.
func (s *Server) RefreshSandboxToken(ctx context.Context, _ *gatewayv1.RefreshSandboxTokenRequest) (*gatewayv1.RefreshSandboxTokenResponse, error) {
	principal, ok := sandboxPrincipalFromContext(ctx)
	if !ok || s.Tokens == nil || s.Sandboxes == nil {
		return nil, status.Error(codes.Unauthenticated, "sandbox authentication required")
	}
	current, err := s.Sandboxes.GetBySandboxID(ctx, principal.sandbox)
	if err != nil || current.RuntimeGenerationOrInitial() != principal.generation || credentialEpoch(current.CredentialEpoch) != principal.credentialEpoch || current.State != "running" {
		return nil, status.Error(codes.Unauthenticated, "sandbox credential is stale")
	}
	// The inbound bearer was verified by the auth interceptor. Mint a fresh
	// generation-bound bearer with the token service's configured lifetime.
	ref := domain.SandboxRef{Workspace: current.Workspace, Sandbox: current.Sandbox}
	epoch := credentialEpoch(current.CredentialEpoch)
	raw, claims, err := s.Tokens.IssueForAudienceEpoch(ref, principal.generation, epoch, "gateway", principal.subject, 0)
	if err != nil {
		return nil, status.Error(codes.Internal, "sandbox token refresh failed")
	}
	sandboxToken, sandboxClaims, err := s.Tokens.IssueForAudienceEpoch(ref, principal.generation, epoch, "sandbox", principal.subject, 0)
	if err != nil {
		return nil, status.Error(codes.Internal, "sandbox protocol credential refresh failed")
	}
	exp := timestamppb.New(claims.ExpiresAt)
	return &gatewayv1.RefreshSandboxTokenResponse{
		Token: raw, ExpirationTime: exp, SandboxToken: sandboxToken,
		SandboxExpirationTime: timestamppb.New(sandboxClaims.ExpiresAt),
		SessionId:             fmt.Sprintf("g%016x", uint64(principal.generation)),
		CredentialEpoch:       epoch,
	}, nil
}

func (s *Server) ConnectSupervisor(stream gatewayv1.OpenShell_ConnectSupervisorServer) error {
	principal, ok := sandboxPrincipalFromContext(stream.Context())
	if !ok || s.Sandboxes == nil || s.SupervisorHub == nil {
		return status.Error(codes.Unauthenticated, "sandbox authentication required")
	}
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil || hello.GetSandboxId() != string(principal.sandbox) || strings.TrimSpace(hello.GetInstanceId()) == "" || hello.GetConnectionEpoch() == 0 {
		return status.Error(codes.PermissionDenied, "supervisor hello does not match credential")
	}
	current, err := s.Sandboxes.GetBySandboxID(stream.Context(), principal.sandbox)
	if err != nil || current.RuntimeGenerationOrInitial() != principal.generation || credentialEpoch(current.CredentialEpoch) != principal.credentialEpoch || current.State != "running" {
		return status.Error(codes.PermissionDenied, "sandbox generation is not running")
	}
	sessionID := randomSessionID()
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	done := make(chan struct{})
	var sendMu sync.Mutex
	send := func(msg *gatewayv1.GatewayMessage) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(msg)
	}
	unregister, err := s.SupervisorHub.RegisterSupervisor(string(principal.sandbox), hello.GetInstanceId(), hello.GetConnectionEpoch(), func(channel, target string) error {
		relay, err := relayOpenMessage(channel, target)
		if err != nil {
			return err
		}
		return send(&gatewayv1.GatewayMessage{Payload: &gatewayv1.GatewayMessage_RelayOpen{RelayOpen: relay}})
	}, done, cancel)
	if err != nil {
		_ = stream.Send(&gatewayv1.GatewayMessage{Payload: &gatewayv1.GatewayMessage_SessionRejected{SessionRejected: &gatewayv1.SessionRejected{Reason: err.Error()}}})
		return nil
	}
	defer unregister()
	if err := stream.Send(&gatewayv1.GatewayMessage{Payload: &gatewayv1.GatewayMessage_SessionAccepted{SessionAccepted: &gatewayv1.SessionAccepted{SessionId: sessionID, HeartbeatInterval: durationpb.New(15 * time.Second)}}}); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if send(&gatewayv1.GatewayMessage{Payload: &gatewayv1.GatewayMessage_Heartbeat{Heartbeat: &gatewayv1.GatewayHeartbeat{}}}) != nil {
					cancel()
					return
				}
			}
		}
	}()
	for {
		msg, err := stream.Recv()
		if err != nil {
			close(done)
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch payload := msg.GetPayload().(type) {
		case *gatewayv1.SupervisorMessage_Heartbeat:
			if !s.SupervisorHub.RenewSupervisorOwner(string(principal.sandbox), hello.GetInstanceId(), hello.GetConnectionEpoch()) {
				return status.Error(codes.Aborted, "supervisor session ownership expired")
			}
			if err := send(&gatewayv1.GatewayMessage{Payload: &gatewayv1.GatewayMessage_Heartbeat{Heartbeat: &gatewayv1.GatewayHeartbeat{}}}); err != nil {
				return err
			}
		case *gatewayv1.SupervisorMessage_RelayOpenResult:
			if payload.RelayOpenResult != nil && !payload.RelayOpenResult.GetSuccess() {
				s.SupervisorHub.CloseRelay(string(principal.sandbox), payload.RelayOpenResult.GetChannelId(), errors.New(payload.RelayOpenResult.GetError()))
			}
		case *gatewayv1.SupervisorMessage_RelayClose:
			if payload.RelayClose != nil {
				s.SupervisorHub.CloseRelay(string(principal.sandbox), payload.RelayClose.GetChannelId(), errors.New(payload.RelayClose.GetReason()))
			}
		default:
			return status.Error(codes.InvalidArgument, "unexpected supervisor message")
		}
	}
}

func relayOpenMessage(channel, target string) (*gatewayv1.RelayOpen, error) {
	out := &gatewayv1.RelayOpen{ChannelId: channel}
	if target == "ssh" {
		out.Target = &gatewayv1.RelayOpen_Ssh{Ssh: &gatewayv1.SshRelayTarget{}}
		return out, nil
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "tcp" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid relay target")
	}
	host := strings.Trim(u.Hostname(), "[]")
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil || port == 0 || (host != "localhost" && net.ParseIP(host) == nil) || (host != "localhost" && !net.ParseIP(host).IsLoopback()) {
		return nil, errors.New("relay target must be loopback TCP")
	}
	out.Target = &gatewayv1.RelayOpen_Tcp{Tcp: &gatewayv1.TcpRelayTarget{Host: host, Port: uint32(port)}}
	return out, nil
}

func (s *Server) RelayStream(stream gatewayv1.OpenShell_RelayStreamServer) error {
	principal, ok := sandboxPrincipalFromContext(stream.Context())
	if !ok || s.Sandboxes == nil || s.SupervisorHub == nil {
		return status.Error(codes.Unauthenticated, "sandbox authentication required")
	}
	current, err := s.Sandboxes.GetBySandboxID(stream.Context(), principal.sandbox)
	if err != nil || current.RuntimeGenerationOrInitial() != principal.generation || credentialEpoch(current.CredentialEpoch) != principal.credentialEpoch || current.State != "running" {
		return status.Error(codes.PermissionDenied, "sandbox generation is not running")
	}
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	init := first.GetInit()
	if init == nil || init.GetChannelId() == "" {
		return status.Error(codes.InvalidArgument, "first relay frame must initialize a channel")
	}
	conn, err := s.SupervisorHub.AcceptRelay(string(principal.sandbox), init.GetChannelId())
	if err != nil {
		return status.Error(codes.NotFound, "relay channel is not pending for this sandbox")
	}
	defer conn.Close()
	copyDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, readErr := conn.Read(buf)
			if n > 0 {
				if sendErr := stream.Send(&gatewayv1.RelayFrame{Payload: &gatewayv1.RelayFrame_Data{Data: append([]byte(nil), buf[:n]...)}}); sendErr != nil {
					copyDone <- sendErr
					return
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) || errors.Is(readErr, net.ErrClosed) {
					copyDone <- nil
				} else {
					copyDone <- readErr
				}
				return
			}
		}
	}()
	recvDone := make(chan error, 1)
	go func() {
		for {
			frame, recvErr := stream.Recv()
			if recvErr != nil {
				if errors.Is(recvErr, io.EOF) {
					if cw, ok := conn.(interface{ CloseWrite() error }); ok {
						_ = cw.CloseWrite()
					}
					recvDone <- nil
				} else {
					recvDone <- recvErr
				}
				return
			}
			data := frame.GetData()
			if data == nil {
				recvDone <- status.Error(codes.InvalidArgument, "relay frames after init must contain data")
				return
			}
			if _, err := conn.Write(data); err != nil {
				recvDone <- err
				return
			}
		}
	}()
	for {
		select {
		case err := <-recvDone:
			if err != nil {
				return err
			}
			select {
			case err := <-copyDone:
				return err
			case <-stream.Context().Done():
				return stream.Context().Err()
			}
		case err := <-copyDone:
			return err
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func randomSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// CreateSshSession requires the SSH runtime.
func (s *Server) CreateSshSession(context.Context, *gatewayv1.CreateSshSessionRequest) (*gatewayv1.CreateSshSessionResponse, error) {
	return nil, unavailable("ssh session runtime")
}

// RevokeSshSession requires the SSH runtime.
func (s *Server) RevokeSshSession(context.Context, *gatewayv1.RevokeSshSessionRequest) (*gatewayv1.RevokeSshSessionResponse, error) {
	return nil, unavailable("ssh session runtime")
}

// GetSandboxLogs requires the log runtime.
func (s *Server) GetSandboxLogs(context.Context, *gatewayv1.GetSandboxLogsRequest) (*gatewayv1.GetSandboxLogsResponse, error) {
	return nil, unavailable("sandbox log runtime")
}

// PeerReportProviderReadiness requires the peer relay runtime.
func (s *Server) PeerReportProviderReadiness(context.Context, *gatewayv1.ReportProviderReadinessRequest) (*gatewayv1.ReportProviderReadinessResponse, error) {
	return nil, unavailable("peer provider readiness")
}

// PeerReportEndpointStatus requires the peer relay runtime.
func (s *Server) PeerReportEndpointStatus(context.Context, *gatewayv1.ReportEndpointStatusRequest) (*gatewayv1.ReportEndpointStatusResponse, error) {
	return nil, unavailable("peer endpoint status")
}

// PeerGetSandboxProviderStatus requires the peer relay runtime.
func (s *Server) PeerGetSandboxProviderStatus(context.Context, *gatewayv1.GetSandboxProviderStatusRequest) (*gatewayv1.GetSandboxProviderStatusResponse, error) {
	return nil, unavailable("peer sandbox provider status")
}
