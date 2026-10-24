package credentialstore

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	credentialv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/credentials/v1"
	datamodelv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	extensionv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/extension/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func newTestGRPCDriver(t *testing.T) (*GRPCDriver, *MemoryStore) {
	t.Helper()
	store, err := NewMemoryStore([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := NewGRPCDriver("test-driver", "test-version", store)
	if err != nil {
		t.Fatal(err)
	}
	return driver, store
}

func TestCredentialDriverCapabilitiesNegotiation(t *testing.T) {
	driver, _ := newTestGRPCDriver(t)
	response, err := driver.GetCapabilities(context.Background(), &credentialv1.GetCredentialDriverCapabilitiesRequest{
		Gateway: &extensionv1.PeerMetadata{
			ProtocolVersion:      &extensionv1.ProtocolVersion{Major: 1},
			RequiredCapabilities: []string{credentialContractCapability},
		},
	})
	if err != nil || response.GetDriverName() != "test-driver" || response.GetSupportsList() || !response.GetSupportsExpiresAt() {
		t.Fatalf("GetCapabilities() = %+v, %v", response, err)
	}
	if response.GetExtension().GetProtocolVersion().GetMajor() != 1 || !strings.Contains(strings.Join(response.GetExtension().GetSupportedCapabilities(), ","), credentialContractCapability) {
		t.Fatalf("credential protocol metadata = %+v", response.GetExtension())
	}
	_, err = driver.GetCapabilities(context.Background(), &credentialv1.GetCredentialDriverCapabilitiesRequest{
		Gateway: &extensionv1.PeerMetadata{ProtocolVersion: &extensionv1.ProtocolVersion{Major: 2}},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unsupported protocol = %v", err)
	}
	if _, err := driver.GetCapabilities(context.Background(), &credentialv1.GetCredentialDriverCapabilitiesRequest{
		Gateway: &extensionv1.PeerMetadata{ProtocolVersion: &extensionv1.ProtocolVersion{Major: 1, Minor: 9}},
	}); err != nil {
		t.Fatalf("compatible minor protocol version = %v", err)
	}
	_, err = driver.GetCapabilities(context.Background(), &credentialv1.GetCredentialDriverCapabilitiesRequest{
		Gateway: &extensionv1.PeerMetadata{
			ProtocolVersion:      &extensionv1.ProtocolVersion{Major: 1},
			RequiredCapabilities: []string{"openshell.credentials.unknown"},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unsupported capability = %v", err)
	}
}

func TestCredentialDriverStoreResolveDeleteAndScope(t *testing.T) {
	ctx := context.Background()
	driver, _ := newTestGRPCDriver(t)
	storeRequest := &credentialv1.StoreCredentialRequest{
		Provider: "anthropic", CredentialKey: "ANTHROPIC_API_KEY", Value: "secret-material",
		Workspace: "workspace-a", ProviderId: "provider-uuid-a",
	}
	stored, err := driver.StoreCredential(ctx, storeRequest)
	if err != nil || stored.GetHandle().GetHandle() == "" {
		t.Fatalf("StoreCredential() = %+v, %v", stored, err)
	}
	if strings.Contains(stored.GetHandle().String(), "secret-material") {
		t.Fatalf("credential material leaked in handle: %s", stored.GetHandle())
	}
	resolve := func(requestID, workspace string, handle *datamodelv1.CredentialHandle) (*credentialv1.ResolveCredentialsResponse, error) {
		return driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{{
			RequestId: requestID, Provider: "anthropic", CredentialKey: "ANTHROPIC_API_KEY", Handle: handle,
			Workspace: workspace, ProviderId: "provider-uuid-a",
		}}})
	}
	resolved, err := resolve("request-1", "workspace-a", stored.GetHandle())
	if err != nil || len(resolved.GetCredentials()) != 1 || resolved.GetCredentials()[0].GetValue() != "secret-material" || resolved.GetCredentials()[0].GetRequestId() != "request-1" {
		t.Fatalf("ResolveCredentials() = %+v, %v", resolved, err)
	}
	if _, err := resolve("request-2", "workspace-b", stored.GetHandle()); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-workspace resolve = %v", err)
	}

	storeRequest.Value = "rotated-secret"
	storeRequest.ExistingHandle = stored.GetHandle()
	updated, err := driver.StoreCredential(ctx, storeRequest)
	if err != nil || updated.GetHandle().GetHandle() != stored.GetHandle().GetHandle() {
		t.Fatalf("overwrite = %+v, %v", updated, err)
	}
	resolved, err = resolve("request-3", "workspace-a", updated.GetHandle())
	if err != nil || resolved.GetCredentials()[0].GetValue() != "rotated-secret" {
		t.Fatalf("Resolve after overwrite = %+v, %v", resolved, err)
	}

	_, err = driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{
		{RequestId: "duplicate", Provider: "anthropic", CredentialKey: "ANTHROPIC_API_KEY", Handle: updated.GetHandle(), Workspace: "workspace-a", ProviderId: "provider-uuid-a"},
		{RequestId: "duplicate", Provider: "anthropic", CredentialKey: "ANTHROPIC_API_KEY", Handle: updated.GetHandle(), Workspace: "workspace-a", ProviderId: "provider-uuid-a"},
	}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("duplicate request IDs = %v", err)
	}

	if _, err := driver.DeleteCredential(ctx, &credentialv1.DeleteCredentialRequest{
		Provider: "anthropic", CredentialKey: "ANTHROPIC_API_KEY", Handle: updated.GetHandle(),
		Workspace: "workspace-a", ProviderId: "provider-uuid-a",
	}); err != nil {
		t.Fatalf("DeleteCredential() = %v", err)
	}
	if _, err := resolve("request-4", "workspace-a", updated.GetHandle()); status.Code(err) != codes.NotFound {
		t.Fatalf("resolve after delete = %v", err)
	}
}

func TestCredentialDriverRejectsForgedAndExpiredHandles(t *testing.T) {
	ctx := context.Background()
	driver, store := newTestGRPCDriver(t)
	request := &credentialv1.StoreCredentialRequest{
		Provider: "api", CredentialKey: "TOKEN", Value: "material", Workspace: "ws", ProviderId: "provider-id",
	}
	stored, err := driver.StoreCredential(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	forged := *stored.GetHandle()
	forged.Metadata = map[string]string{"workspace": "other", "provider": "api", "provider_id": "provider-id", "object_id": "provider-id", "credential_key": "TOKEN"}
	_, err = driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{{
		RequestId: "forged", Provider: "api", CredentialKey: "TOKEN", Handle: &forged, Workspace: "other", ProviderId: "provider-id",
	}}})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("forged handle = %v", err)
	}

	validIdentity := credentialIdentity{Workspace: "ws", Provider: "api", ProviderID: "provider-id", ObjectID: "provider-id", Key: "EXPIRING"}
	expiresAt := time.Now().Add(time.Hour).UTC()
	_, err = store.Put(ctx, Secret{ID: domain.CredentialID(validIdentity.handle()), Provider: "api", Material: []byte("valid-expiry"), ExpiresAt: expiresAt})
	if err != nil {
		t.Fatal(err)
	}
	response, err := driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{{
		RequestId: "valid-expiry", Provider: "api", CredentialKey: "EXPIRING", Handle: makeHandle("test-driver", validIdentity), Workspace: "ws", ProviderId: "provider-id",
	}}})
	if err != nil || response.GetCredentials()[0].GetExpirationTime() == nil || response.GetCredentials()[0].GetExpirationTime().AsTime().Unix() != expiresAt.Unix() {
		t.Fatalf("expiry metadata = %+v, %v", response, err)
	}

	identity := credentialIdentity{Workspace: "ws", Provider: "api", ProviderID: "provider-id", ObjectID: "provider-id", Key: "EXPIRED"}
	_, err = store.Put(ctx, Secret{ID: domain.CredentialID(identity.handle()), Provider: "api", Material: []byte("expired"), ExpiresAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{{
		RequestId: "expired", Provider: "api", CredentialKey: "EXPIRED", Handle: makeHandle("test-driver", identity), Workspace: "ws", ProviderId: "provider-id",
	}}})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expired credential resolve = %v", err)
	}
}

func TestCredentialDriverDoesNotReturnPartialBatchOnMissingCredential(t *testing.T) {
	ctx := context.Background()
	driver, _ := newTestGRPCDriver(t)
	goodRequest := &credentialv1.StoreCredentialRequest{Provider: "api", CredentialKey: "GOOD", Value: "material", Workspace: "ws", ProviderId: "provider-id"}
	good, err := driver.StoreCredential(ctx, goodRequest)
	if err != nil {
		t.Fatal(err)
	}
	missing := makeHandle("test-driver", credentialIdentity{Workspace: "ws", Provider: "api", ProviderID: "provider-id", ObjectID: "provider-id", Key: "MISSING"})
	response, err := driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{
		{RequestId: "good", Provider: "api", CredentialKey: "GOOD", Handle: good.GetHandle(), Workspace: "ws", ProviderId: "provider-id"},
		{RequestId: "missing", Provider: "api", CredentialKey: "MISSING", Handle: missing, Workspace: "ws", ProviderId: "provider-id"},
	}})
	if response != nil || status.Code(err) != codes.NotFound {
		t.Fatalf("partial batch = %+v, %v", response, err)
	}
	if got := grpcCredentialError(domain.Wrap(domain.ErrUnavailable, "store", "secret")); status.Code(got) != codes.Unavailable || strings.Contains(got.Error(), "secret") {
		t.Fatalf("safe outage mapping = %v", got)
	}
	if got := grpcCredentialError(errors.New("private backend detail")); strings.Contains(got.Error(), "private backend detail") {
		t.Fatalf("backend error leaked: %v", got)
	}
}

func TestCredentialDriverStagesDistinctObjectWithoutReplacingCurrentValue(t *testing.T) {
	ctx := context.Background()
	driver, _ := newTestGRPCDriver(t)
	current, err := driver.StoreCredential(ctx, &credentialv1.StoreCredentialRequest{
		Provider: "api", CredentialKey: "TOKEN", Value: "current", Workspace: "ws", ProviderId: "provider-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := driver.StoreCredential(ctx, &credentialv1.StoreCredentialRequest{
		Provider: "api", CredentialKey: "TOKEN", Value: "staged", Workspace: "ws", ProviderId: "provider-id",
		ObjectId: "provider-id-refresh-2", ExistingHandle: current.GetHandle(),
	})
	if err != nil || staged.GetHandle().GetHandle() == current.GetHandle().GetHandle() {
		t.Fatalf("staged StoreCredential() = %+v, %v", staged, err)
	}
	resolve := func(id string, handle *datamodelv1.CredentialHandle) string {
		t.Helper()
		response, err := driver.ResolveCredentials(ctx, &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{{
			RequestId: id, Provider: "api", CredentialKey: "TOKEN", Handle: handle, Workspace: "ws", ProviderId: "provider-id",
		}}})
		if err != nil {
			t.Fatalf("Resolve(%s): %v", id, err)
		}
		return response.GetCredentials()[0].GetValue()
	}
	if got := resolve("current", current.GetHandle()); got != "current" {
		t.Fatalf("current handle resolved %q", got)
	}
	if got := resolve("staged", staged.GetHandle()); got != "staged" {
		t.Fatalf("staged handle resolved %q", got)
	}
}

func TestCredentialDriverGRPCRoundTrip(t *testing.T) {
	driver, _ := newTestGRPCDriver(t)
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	credentialv1.RegisterCredentialDriverServer(server, driver)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	connection, err := grpc.NewClient("passthrough:///credential-driver", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := credentialv1.NewCredentialDriverClient(connection)
	capabilities, err := client.GetCapabilities(t.Context(), &credentialv1.GetCredentialDriverCapabilitiesRequest{
		Gateway: &extensionv1.PeerMetadata{ProtocolVersion: &extensionv1.ProtocolVersion{Major: 1}, RequiredCapabilities: []string{credentialContractCapability}},
	})
	if err != nil || capabilities.GetDriverName() != "test-driver" {
		t.Fatalf("GetCapabilities RPC = %+v, %v", capabilities, err)
	}
	stored, err := client.StoreCredential(t.Context(), &credentialv1.StoreCredentialRequest{
		Provider: "api", CredentialKey: "TOKEN", Value: "round-trip", Workspace: "ws", ProviderId: "provider-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := client.ResolveCredentials(t.Context(), &credentialv1.ResolveCredentialsRequest{Credentials: []*credentialv1.ResolveCredentialRequest{{
		RequestId: "round-trip", Provider: "api", CredentialKey: "TOKEN", Handle: stored.GetHandle(), Workspace: "ws", ProviderId: "provider-id",
	}}})
	if err != nil || resolved.GetCredentials()[0].GetValue() != "round-trip" {
		t.Fatalf("ResolveCredentials RPC = %+v, %v", resolved, err)
	}
	if _, err := client.DeleteCredential(t.Context(), &credentialv1.DeleteCredentialRequest{
		Provider: "api", CredentialKey: "TOKEN", Handle: stored.GetHandle(), Workspace: "ws", ProviderId: "provider-id",
	}); err != nil {
		t.Fatalf("DeleteCredential RPC = %v", err)
	}
	if _, err := client.ListCredentials(t.Context(), &credentialv1.ListCredentialsRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("optional ListCredentials RPC = %v", err)
	}
}
