package httpproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-credentials"
	"github.com/KLORQ/quarshell-proxy/internal/application/mediation"
)

func TestProxyInjectsCredentialHeader(t *testing.T) {
	var authorization string
	var leaked http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		leaked = request.Header.Clone()
		_, _ = io.WriteString(writer, "ok")
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(upstreamURL.Port())
	store, err := credentials.NewMemoryStore([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	endpoint := credentials.EndpointBinding{Scheme: "http", Host: upstreamURL.Hostname(), Port: uint16(port), Method: "GET", PathPrefix: "/"}
	if _, err := store.Put(context.Background(), credentials.Secret{
		ID: "cred", Provider: "api", Material: []byte("token-123"), Endpoint: endpoint, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	broker, _ := credentials.NewBroker(store, nil)
	handle, _, err := broker.Issue(context.Background(), credentials.IssueRequest{
		Credential: "cred", Sandbox: domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}, Endpoint: endpoint,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	mediator := &mediation.Mediator{
		Policies: staticPolicy{snapshot: snapshotFor(t, fmt.Sprintf(`version: 1
network_policies:
  upstream:
    endpoints:
      - host: %s
        port: %s
        protocol: rest
        access: full
`, upstreamURL.Hostname(), upstreamURL.Port()))},
		Rewriter: &credentials.Rewriter{Broker: broker},
		Dialer:   &HTTPDialer{},
	}
	server := New(mediator)
	proxy := httptest.NewServer(server)
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	request, _ := http.NewRequest(http.MethodGet, upstream.URL+"/hello", nil)
	request.Header.Set("X-Quarshell-Credential", string(handle))
	request.Header.Set("X-Quarshell-Credential-Header", "Authorization")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if authorization != "Bearer token-123" {
		t.Fatalf("authorization = %q", authorization)
	}
	if leaked.Get("X-Quarshell-Credential") != "" || leaked.Get("X-Quarshell-Credential-Header") != "" {
		t.Fatalf("control headers leaked upstream: %v", leaked)
	}
}
