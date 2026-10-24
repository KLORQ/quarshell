package applicationgrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	"github.com/KLORQ/quarshell-gateway/internal/application/workspace"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	gatewayv1connect "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1/gatewayv1connect"
)

func TestConnectHTTPGatewaySharesBearerAuthAndHealthRules(t *testing.T) {
	workspaces, err := workspace.NewService(lifecycle.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	application := New("test-version", workspaces, nil, nil, nil, nil, nil, nil)
	adapter := &OpenShellConnectAdapter{Server: application}
	path, handler := gatewayv1connect.NewOpenShellHandler(adapter, connect.WithInterceptors(ConnectAuthInterceptor("secret-token", "operator", nil)))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	unauthenticated := gatewayv1connect.NewOpenShellClient(server.Client(), server.URL)
	if _, err := unauthenticated.GetGatewayInfo(context.Background(), connect.NewRequest(&gatewayv1.GetGatewayInfoRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unauthenticated GetGatewayInfo code=%s, want unauthenticated (err=%v)", connect.CodeOf(err), err)
	}
	if _, err := unauthenticated.Health(context.Background(), connect.NewRequest(&gatewayv1.HealthRequest{})); err != nil {
		t.Fatalf("public Connect health: %v", err)
	}

	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		request = request.Clone(request.Context())
		request.Header = request.Header.Clone()
		request.Header.Set("Authorization", "Bearer secret-token")
		return http.DefaultTransport.RoundTrip(request)
	})
	authenticatedHTTP := &http.Client{Transport: transport}
	authenticated := gatewayv1connect.NewOpenShellClient(authenticatedHTTP, server.URL)
	user, err := authenticated.GetCurrentUser(context.Background(), connect.NewRequest(&gatewayv1.GetCurrentUserRequest{}))
	if err != nil || user.Msg.GetSubject() != "operator" {
		t.Fatalf("authenticated Connect GetCurrentUser=%+v err=%v", user, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
