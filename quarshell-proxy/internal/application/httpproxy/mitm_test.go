package httpproxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-policy"
	"github.com/KLORQ/quarshell-proxy/internal/application/mediation"
)

func newMITMProxy(t *testing.T, snapshot *policy.Snapshot) (*Server, *httptest.Server, *CA) {
	t.Helper()
	ca, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA: %v", err)
	}
	upstreamClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // test upstream uses its own self-signed certificate
		}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	mediator := &mediation.Mediator{Policies: staticPolicy{snapshot: snapshot}, Dialer: &HTTPDialer{Client: upstreamClient}}
	server := New(mediator)
	server.MITM = true
	server.CA = ca
	return server, httptest.NewServer(server), ca
}

func mitmClient(t *testing.T, proxy *httptest.Server, ca *CA) *http.Client {
	t.Helper()
	proxyURL, _ := url.Parse(proxy.URL)
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{RootCAs: ca.Pool()},
		},
	}
}

func TestMITMInterceptsHTTPS(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Mitm", "yes")
		_, _ = io.WriteString(writer, "secure upstream")
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	_, proxy, ca := newMITMProxy(t, snapshotFor(t, fmt.Sprintf(`version: 1
network_policies:
  upstream:
    endpoints:
      - host: %s
        port: %s
        protocol: rest
        access: full
`, upstreamURL.Hostname(), upstreamURL.Port())))
	defer proxy.Close()
	response, err := mitmClient(t, proxy, ca).Get(upstream.URL + "/hello")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "secure upstream" || response.Header.Get("X-Mitm") != "yes" {
		t.Fatalf("response = %d %q headers=%v", response.StatusCode, body, response.Header)
	}
}

func TestMITMDeniesUnlistedHost(t *testing.T) {
	_, proxy, ca := newMITMProxy(t, snapshotFor(t, `version: 1
network_policies:
  other:
    endpoints:
      - host: allowed.example
        port: 443
        protocol: rest
        access: full
`))
	defer proxy.Close()
	response, err := mitmClient(t, proxy, ca).Get("https://denied.example/")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestMITMBodyLimit(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	server, proxy, ca := newMITMProxy(t, snapshotFor(t, fmt.Sprintf(`version: 1
network_policies:
  upstream:
    endpoints:
      - host: %s
        port: %s
        protocol: rest
        access: full
`, upstreamURL.Hostname(), upstreamURL.Port())))
	defer proxy.Close()
	server.MaxBodyBytes = 4
	request, _ := http.NewRequest(http.MethodPost, upstream.URL+"/", strings.NewReader("0123456789"))
	response, err := mitmClient(t, proxy, ca).Do(request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", response.StatusCode)
	}
}
