package applicationgrpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	policyapp "github.com/KLORQ/quarshell-gateway/internal/application/policy"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPolicyPageSize = 100
	maxPolicyPageSize     = 1000
)

// ListSandboxPolicies returns revision metadata without exposing policy source.
func (s *Server) ListSandboxPolicies(ctx context.Context, request *gatewayv1.ListSandboxPoliciesRequest) (*gatewayv1.ListSandboxPoliciesResponse, error) {
	if s.PolicyRevisions == nil {
		return nil, status.Error(codes.Unavailable, "policy revision service is not configured")
	}
	if request.GetGlobal() {
		return nil, status.Error(codes.Unimplemented, "global policy history is not migrated")
	}
	sandbox := strings.TrimSpace(request.GetSandbox())
	if sandbox == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox is required")
	}
	pageSize := int(request.GetPageSize())
	if pageSize < 0 {
		return nil, status.Error(codes.InvalidArgument, "page_size must not be negative")
	}
	if pageSize == 0 {
		pageSize = defaultPolicyPageSize
	} else if pageSize > maxPolicyPageSize {
		pageSize = maxPolicyPageSize
	}
	scope := policyScopeFor(request.GetWorkspaceScope(), sandbox)
	if !scope.Valid() {
		return nil, status.Error(codes.InvalidArgument, "workspace scope is required")
	}
	offset, err := decodePolicyPageToken(request.GetPageToken(), scope.Key())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	revisions, err := s.PolicyRevisions.List(ctx, scope)
	if err != nil {
		return nil, grpcError(err)
	}
	if offset > len(revisions) {
		return nil, status.Error(codes.InvalidArgument, "page_token is out of range")
	}
	end := offset + pageSize
	if end > len(revisions) {
		end = len(revisions)
	}
	response := &gatewayv1.ListSandboxPoliciesResponse{Revisions: make([]*gatewayv1.SandboxPolicyRevision, 0, end-offset)}
	for _, revision := range revisions[offset:end] {
		response.Revisions = append(response.Revisions, sandboxPolicyRevisionProto(revision))
	}
	if end < len(revisions) {
		response.NextPageToken = encodePolicyPageToken(scope.Key(), end)
	}
	return response, nil
}

// GetSandboxPolicyStatus returns one persisted revision and the observed active version.
func (s *Server) GetSandboxPolicyStatus(ctx context.Context, request *gatewayv1.GetSandboxPolicyStatusRequest) (*gatewayv1.GetSandboxPolicyStatusResponse, error) {
	if s.PolicyRevisions == nil {
		return nil, status.Error(codes.Unavailable, "policy revision service is not configured")
	}
	if request.GetGlobal() {
		return nil, status.Error(codes.Unimplemented, "global policy history is not migrated")
	}
	sandbox := strings.TrimSpace(request.GetSandbox())
	if sandbox == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox is required")
	}
	scope := policyScopeFor(request.GetWorkspaceScope(), sandbox)
	if !scope.Valid() {
		return nil, status.Error(codes.InvalidArgument, "workspace scope is required")
	}
	revisions, err := s.PolicyRevisions.List(ctx, scope)
	if err != nil {
		return nil, grpcError(err)
	}
	var selected *policyapp.Revision
	if request.GetVersion() == 0 {
		if len(revisions) > 0 {
			selected = &revisions[0]
		}
	} else {
		for index := range revisions {
			if revisions[index].Number == uint64(request.GetVersion()) {
				selected = &revisions[index]
				break
			}
		}
	}
	if selected == nil {
		return nil, status.Error(codes.NotFound, "policy revision not found")
	}
	response := &gatewayv1.GetSandboxPolicyStatusResponse{Revision: sandboxPolicyRevisionProto(*selected)}
	active, exists, err := s.PolicyRevisions.Active(ctx, scope)
	if err != nil {
		return nil, grpcError(err)
	}
	if exists && active.ObservedAt != nil {
		response.ActiveVersion = uint32(active.Number)
	}
	return response, nil
}

func sandboxPolicyRevisionProto(revision policyapp.Revision) *gatewayv1.SandboxPolicyRevision {
	statusValue := gatewayv1.PolicyStatus_POLICY_STATUS_PENDING
	switch revision.Status {
	case policyapp.StatusActive:
		if revision.ObservedAt != nil {
			statusValue = gatewayv1.PolicyStatus_POLICY_STATUS_LOADED
		}
	case policyapp.StatusSuperseded:
		statusValue = gatewayv1.PolicyStatus_POLICY_STATUS_SUPERSEDED
	case policyapp.StatusRejected:
		statusValue = gatewayv1.PolicyStatus_POLICY_STATUS_UNSPECIFIED
	}
	item := &gatewayv1.SandboxPolicyRevision{
		Version: uint32(revision.Number), PolicyHash: revision.Hash,
		Status: statusValue, CreatedTime: timestamppb.New(revision.CreatedAt),
	}
	if revision.ObservedAt != nil {
		item.LoadedTime = timestamppb.New(*revision.ObservedAt)
	}
	return item
}

func encodePolicyPageToken(scope string, offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s\n%d", scope, offset)))
}

func decodePolicyPageToken(token, scope string) (int, error) {
	if token == "" {
		return 0, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(string(payload), "\n")
	if len(parts) != 2 || parts[0] != scope {
		return 0, fmt.Errorf("page token scope mismatch")
	}
	offset, err := strconv.Atoi(parts[1])
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid page token offset")
	}
	return offset, nil
}
