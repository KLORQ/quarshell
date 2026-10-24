package credentialstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	credentialv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/credentials/v1"
	datamodelv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	extensionv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/extension/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const credentialContractCapability = "openshell.credentials.contract"

// GRPCDriver serves the OpenShell credential-driver contract using an encrypted
// credential store. Handles are scoped to workspace, provider, object and key.
type GRPCDriver struct {
	credentialv1.UnimplementedCredentialDriverServer
	name    string
	version string
	store   Store
}

// NewGRPCDriver creates a credential-driver server. ListCredentials is not
// supported by the underlying store and is advertised as such.
func NewGRPCDriver(name, version string, store Store) (*GRPCDriver, error) {
	if strings.TrimSpace(name) == "" || store == nil {
		return nil, domain.Invalidf("credentials.grpc", "driver name and store are required")
	}
	return &GRPCDriver{name: strings.TrimSpace(name), version: strings.TrimSpace(version), store: store}, nil
}

// GetCapabilities reports protocol and backend support.
func (d *GRPCDriver) GetCapabilities(_ context.Context, request *credentialv1.GetCredentialDriverCapabilitiesRequest) (*credentialv1.GetCredentialDriverCapabilitiesResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "gateway protocol metadata is required")
	}
	if request.GetGateway() == nil || request.GetGateway().GetProtocolVersion() == nil {
		return nil, status.Error(codes.InvalidArgument, "gateway protocol metadata is required")
	}
	version := request.GetGateway().GetProtocolVersion()
	if version.GetMajor() != 1 {
		return nil, status.Error(codes.FailedPrecondition, "unsupported credential-driver protocol version")
	}
	for _, required := range request.GetGateway().GetRequiredCapabilities() {
		if required != credentialContractCapability {
			return nil, status.Error(codes.FailedPrecondition, "gateway requires an unsupported credential capability")
		}
	}
	return &credentialv1.GetCredentialDriverCapabilitiesResponse{
		DriverName: d.name, DriverVersion: d.version, BackendKind: "encrypted-store",
		SupportsList: false, SupportsExpiresAt: true,
		Extension: &extensionv1.PeerMetadata{
			ProtocolVersion:    &extensionv1.ProtocolVersion{Major: 1},
			ImplementationName: "quarshell/credential-store", ImplementationVersion: d.version,
			SupportedCapabilities: []string{credentialContractCapability},
		},
	}, nil
}

// StoreCredential persists one secret and returns its scoped opaque handle.
func (d *GRPCDriver) StoreCredential(ctx context.Context, request *credentialv1.StoreCredentialRequest) (*credentialv1.StoreCredentialResponse, error) {
	if request == nil || request.GetWorkspace() == "" || request.GetProvider() == "" || request.GetProviderId() == "" || request.GetCredentialKey() == "" || request.GetValue() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace, provider, provider_id, credential_key and value are required")
	}
	objectID := request.GetObjectId()
	identity := credentialIdentity{
		Workspace: request.GetWorkspace(), Provider: request.GetProvider(), ProviderID: request.GetProviderId(),
		ObjectID: objectID, Key: request.GetCredentialKey(),
	}
	var err error
	if existing := request.GetExistingHandle(); existing != nil {
		if err := validateHandle(existing, d.name, credentialIdentity{
			Workspace: identity.Workspace, Provider: identity.Provider, ProviderID: identity.ProviderID, Key: identity.Key,
		}); err != nil {
			return nil, err
		}
		if objectID == "" {
			identity, err = identityFromHandle(existing, d.name)
			if err != nil {
				return nil, err
			}
		}
	}
	if identity.ObjectID == "" {
		identity.ObjectID = identity.ProviderID
	}
	id := domain.CredentialID(identity.handle())
	secret, err := d.store.Get(ctx, id)
	switch {
	case err == nil:
		secret, err = d.store.Rotate(ctx, id, []byte(request.GetValue()), secret.ExpiresAt)
	case errors.Is(err, domain.ErrNotFound):
		_, err = d.store.Put(ctx, Secret{ID: id, Provider: domain.ProviderID(request.GetProvider()), Material: []byte(request.GetValue())})
		if err != nil && errors.Is(err, domain.ErrConflict) {
			secret, err = d.store.Get(ctx, id)
			if err == nil {
				_, err = d.store.Rotate(ctx, id, []byte(request.GetValue()), secret.ExpiresAt)
			}
		}
	}
	if err != nil {
		return nil, grpcCredentialError(err)
	}
	return &credentialv1.StoreCredentialResponse{Handle: makeHandle(d.name, identity)}, nil
}

// DeleteCredential removes a scoped handle. Repeated deletion is successful.
func (d *GRPCDriver) DeleteCredential(ctx context.Context, request *credentialv1.DeleteCredentialRequest) (*credentialv1.DeleteCredentialResponse, error) {
	if request == nil || request.GetHandle() == nil {
		return nil, status.Error(codes.InvalidArgument, "credential handle is required")
	}
	identity := credentialIdentity{Workspace: request.GetWorkspace(), Provider: request.GetProvider(), ProviderID: request.GetProviderId(), Key: request.GetCredentialKey()}
	if err := validateHandle(request.GetHandle(), d.name, identity); err != nil {
		return nil, err
	}
	storedIdentity, err := identityFromHandle(request.GetHandle(), d.name)
	if err != nil {
		return nil, err
	}
	if err := d.store.Delete(ctx, domain.CredentialID(storedIdentity.handle())); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, grpcCredentialError(err)
	}
	return &credentialv1.DeleteCredentialResponse{}, nil
}

// ResolveCredentials resolves a batch only when every handle belongs to the
// supplied workspace/provider/key tuple.
func (d *GRPCDriver) ResolveCredentials(ctx context.Context, request *credentialv1.ResolveCredentialsRequest) (*credentialv1.ResolveCredentialsResponse, error) {
	if request == nil || len(request.GetCredentials()) == 0 || len(request.GetCredentials()) > 128 {
		return nil, status.Error(codes.InvalidArgument, "1..128 credentials are required")
	}
	seen := make(map[string]struct{}, len(request.GetCredentials()))
	resolved := make([]*credentialv1.ResolvedCredential, 0, len(request.GetCredentials()))
	for _, item := range request.GetCredentials() {
		if item == nil || item.GetRequestId() == "" || item.GetHandle() == nil || item.GetWorkspace() == "" || item.GetProvider() == "" || item.GetProviderId() == "" || item.GetCredentialKey() == "" {
			return nil, status.Error(codes.InvalidArgument, "request_id, scope, credential_key and handle are required")
		}
		if _, duplicate := seen[item.GetRequestId()]; duplicate {
			return nil, status.Error(codes.InvalidArgument, "request_id values must be unique")
		}
		seen[item.GetRequestId()] = struct{}{}
		identity, err := identityFromHandle(item.GetHandle(), d.name)
		if err != nil {
			return nil, err
		}
		if identity.Workspace != item.GetWorkspace() || identity.Provider != item.GetProvider() || identity.ProviderID != item.GetProviderId() || identity.Key != item.GetCredentialKey() {
			return nil, status.Error(codes.PermissionDenied, "credential handle scope does not match request")
		}
		secret, err := d.store.Get(ctx, domain.CredentialID(identity.handle()))
		if err != nil {
			return nil, grpcCredentialError(err)
		}
		if !secret.ExpiresAt.IsZero() && !time.Now().Before(secret.ExpiresAt) {
			return nil, status.Error(codes.NotFound, "credential has expired")
		}
		value := &credentialv1.ResolvedCredential{RequestId: item.GetRequestId(), Value: string(secret.Material)}
		if !secret.ExpiresAt.IsZero() {
			value.ExpirationTime = timestamppb.New(secret.ExpiresAt)
		}
		resolved = append(resolved, value)
	}
	return &credentialv1.ResolveCredentialsResponse{Credentials: resolved}, nil
}

// ListCredentials is optional in the protocol and unsupported by this store.
func (d *GRPCDriver) ListCredentials(context.Context, *credentialv1.ListCredentialsRequest) (*credentialv1.ListCredentialsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "credential listing is not supported")
}

type credentialIdentity struct {
	Workspace  string
	Provider   string
	ProviderID string
	ObjectID   string
	Key        string
}

func (i credentialIdentity) handle() string {
	parts := []string{i.Workspace, i.Provider, i.ProviderID, i.ObjectID, i.Key}
	encoded := strings.Join(parts, "\x00")
	digest := sha256.Sum256([]byte(encoded))
	return "cred_" + hex.EncodeToString(digest[:])
}

func makeHandle(driver string, identity credentialIdentity) *datamodelv1.CredentialHandle {
	objectID := identity.ObjectID
	if objectID == "" {
		objectID = identity.ProviderID
	}
	return &datamodelv1.CredentialHandle{
		Driver: driver, Handle: identity.handle(), Metadata: map[string]string{
			"workspace": identity.Workspace, "provider": identity.Provider,
			"provider_id": identity.ProviderID, "object_id": objectID,
			"credential_key": identity.Key,
		},
	}
}

func validateHandle(handle *datamodelv1.CredentialHandle, driver string, expected credentialIdentity) error {
	identity, err := identityFromHandle(handle, driver)
	if err != nil {
		return err
	}
	if identity.Workspace != expected.Workspace || identity.Provider != expected.Provider || identity.ProviderID != expected.ProviderID || identity.Key != expected.Key || (expected.ObjectID != "" && identity.ObjectID != expected.ObjectID) {
		return status.Error(codes.PermissionDenied, "credential handle scope does not match request")
	}
	return nil
}

func identityFromHandle(handle *datamodelv1.CredentialHandle, driver string) (credentialIdentity, error) {
	if handle == nil || handle.GetDriver() != driver {
		return credentialIdentity{}, status.Error(codes.PermissionDenied, "credential handle belongs to another driver")
	}
	metadata := handle.GetMetadata()
	identity := credentialIdentity{
		Workspace: metadata["workspace"], Provider: metadata["provider"], ProviderID: metadata["provider_id"],
		ObjectID: metadata["object_id"], Key: metadata["credential_key"],
	}
	if identity.Workspace == "" || identity.Provider == "" || identity.ProviderID == "" || identity.ObjectID == "" || identity.Key == "" || handle.GetHandle() != identity.handle() {
		return credentialIdentity{}, status.Error(codes.PermissionDenied, "credential handle is invalid")
	}
	return identity, nil
}

func grpcCredentialError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "credential not found")
	case errors.Is(err, domain.ErrConflict):
		return status.Error(codes.Aborted, "credential update conflicted")
	case errors.Is(err, domain.ErrUnavailable):
		return status.Error(codes.Unavailable, "credential store unavailable")
	case errors.Is(err, domain.ErrCanceled):
		return status.Error(codes.Canceled, "credential operation canceled")
	case errors.Is(err, domain.ErrDeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "credential operation deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "credential operation canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "credential operation deadline exceeded")
	default:
		return status.Error(codes.Internal, fmt.Sprintf("credential operation failed (%s)", stableErrorCode(err)))
	}
}

func stableErrorCode(err error) string {
	if err == nil {
		return "unknown"
	}
	return string(domain.CodeOf(err))
}
