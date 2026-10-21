package httpproxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-policy"
	"github.com/KLORQ/quarshell-proxy/internal/application/mediation"
)

func newAuthProxy(t *testing.T, snapshot *policy.Snapshot, token string) *httptest.Server {
	t.Helper()
	mediator := &mediation.Mediator{Policies: staticPolicy{snapshot: snapshot}, Dialer: &HTTPDialer{}}
	server := New(mediator)
	server.Authorizer = StaticToken{Token: token}
	return httptest.NewServer(server)
}

func TestProxyRequiresAuthorization(t *testing.T) {
	var seen http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = request.Header.Clone()
		_, _ = io.WriteString(writer, "ok")
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	proxy := newAuthProxy(t, snapshotFor(t, fmt.Sprintf(`version: 1
network_policies:
  upstream:
    endpoints:
      - host: %s
        port: %s
        protocol: rest
        access: full
`, upstreamURL.Hostname(), upstreamURL.Port())), "secret")
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)

	unauthenticated := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	response, err := unauthenticated.Get(upstream.URL)
	if err != nil {
		t.Fatalf("unauthenticated Get: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("unauthenticated status = %d", response.StatusCode)
	}

	request, _ := http.NewRequest(http.MethodGet, upstream.URL, nil)
	request.Header.Set("Proxy-Authorization", "Bearer secret")
	request.Header.Set("X-Quarshell-Proxy-Token", "secret")
	authenticated := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	response, err = authenticated.Do(request)
	if err != nil {
		t.Fatalf("authenticated Do: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("authenticated response = %d %q", response.StatusCode, body)
	}
	if seen.Get("Proxy-Authorization") != "" || seen.Get("X-Quarshell-Proxy-Token") != "" {
		t.Fatalf("proxy credentials leaked upstream: %v", seen)
	}
}

func TestProxyAuthorizesCONNECT(t *testing.T) {
	proxy := newAuthProxy(t, snapshotFor(t, `version: 1
network_policies:
  tunnel:
    endpoints:
      - host: localhost
        port: 443
        protocol: tcp
`), "secret")
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	for name, headers := range map[string]string{
		"missing": "",
		"valid":   "Proxy-Authorization: Bearer secret\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			connection, err := net.Dial("tcp", proxyURL.Host)
			if err != nil {
				t.Fatalf("Dial: %v", err)
			}
			defer connection.Close()
			if _, err := fmt.Fprintf(connection, "CONNECT localhost:443 HTTP/1.1\r\nHost: localhost:443\r\n%s\r\n", headers); err != nil {
				t.Fatalf("write CONNECT: %v", err)
			}
			response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodConnect})
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if name == "missing" && response.StatusCode != http.StatusProxyAuthRequired {
				t.Fatalf("missing token status = %d", response.StatusCode)
			}
			if name == "valid" && response.StatusCode == http.StatusProxyAuthRequired {
				t.Fatalf("valid token rejected")
			}
		})
	}
}
