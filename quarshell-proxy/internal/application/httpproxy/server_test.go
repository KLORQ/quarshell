package httpproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-policy"
	"github.com/KLORQ/quarshell-proxy/internal/application/mediation"
)

type staticPolicy struct{ snapshot *policy.Snapshot }

func (s staticPolicy) Current(context.Context) (*policy.Snapshot, error) { return s.snapshot, nil }

func snapshotFor(t *testing.T, source string) *policy.Snapshot {
	t.Helper()
	document, err := policy.Parse(strings.NewReader(source), policy.DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snapshot, err := document.Snapshot(1, time.Now())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return snapshot
}

func newProxy(t *testing.T, snapshot *policy.Snapshot) (*Server, *httptest.Server) {
	t.Helper()
	mediator := &mediation.Mediator{Policies: staticPolicy{snapshot: snapshot}, Dialer: &HTTPDialer{}}
	server := New(mediator)
	return server, httptest.NewServer(server)
}

func proxyClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	proxyURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
}

func TestProxyAllowsAndForwardsHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Upstream", "yes")
		_, _ = io.WriteString(writer, "hello from upstream")
	}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	_, proxy := newProxy(t, snapshotFor(t, fmt.Sprintf(`version: 1
network_policies:
  upstream:
    endpoints:
      - host: %s
        port: %s
        protocol: rest
        access: full
`, upstreamURL.Hostname(), upstreamURL.Port())))
	defer proxy.Close()
	client := proxyClient(t, proxy)
	response, err := client.Get(upstream.URL + "/hello")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "hello from upstream" || response.Header.Get("X-Upstream") != "yes" {
		t.Fatalf("response = %d %q headers=%v", response.StatusCode, body, response.Header)
	}
}

func TestProxyDeniesByDefault(t *testing.T) {
	_, proxy := newProxy(t, snapshotFor(t, `version: 1
network_policies:
  other:
    endpoints:
      - host: allowed.example
        port: 80
        protocol: rest
        access: full
`))
	defer proxy.Close()
	client := proxyClient(t, proxy)
	response, err := client.Get("http://denied.example/")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestProxyBodyLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	upstreamURL, _ := url.Parse(upstream.URL)
	server, proxy := newProxy(t, snapshotFor(t, fmt.Sprintf(`version: 1
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
	client := proxyClient(t, proxy)
	request, _ := http.NewRequest(http.MethodPost, upstream.URL+"/", strings.NewReader("0123456789"))
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestProxyCONNECTTunnel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func(connection net.Conn) {
				defer connection.Close()
				_, _ = io.Copy(connection, connection)
			}(connection)
		}
	}()
	address := listener.Addr().(*net.TCPAddr)
	_, proxy := newProxy(t, snapshotFor(t, fmt.Sprintf(`version: 1
network_policies:
  tunnel:
    endpoints:
      - host: localhost
        port: %d
        protocol: tcp
`, address.Port)))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	connection, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatalf("Dial proxy: %v", err)
	}
	defer connection.Close()
	target := fmt.Sprintf("localhost:%d", address.Port)
	if _, err := fmt.Fprintf(connection, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", response.StatusCode)
	}
	if _, err := connection.Write([]byte("ping")); err != nil {
		t.Fatalf("write tunnel: %v", err)
	}
	buffer := make([]byte, 4)
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(reader, buffer); err != nil {
		t.Fatalf("read tunnel: %v", err)
	}
	if string(buffer) != "ping" {
		t.Fatalf("tunnel echo = %q", buffer)
	}
}

func TestProxyDeniesCONNECT(t *testing.T) {
	_, proxy := newProxy(t, snapshotFor(t, `version: 1
network_policies:
  other:
    endpoints:
      - host: allowed.example
        port: 443
        protocol: tcp
`))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	connection, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatalf("Dial proxy: %v", err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintf(connection, "CONNECT denied.example:443 HTTP/1.1\r\nHost: denied.example:443\r\n\r\n"); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("CONNECT status = %d", response.StatusCode)
	}
}
