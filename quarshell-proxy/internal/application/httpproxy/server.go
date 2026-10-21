// Package httpproxy is a small default-deny HTTP forward proxy. Plain HTTP
// requests go through the mediation policy and L7 evaluator; CONNECT tunnels
// are authorized at the L4 endpoint level and then relayed opaquely.
package httpproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-credentials"
	"github.com/KLORQ/quarshell-proxy/internal/application/mediation"
)

// Server is an http.Handler implementing a forward proxy.
type Server struct {
	Mediator     *mediation.Mediator
	MaxBodyBytes int
	DialTimeout  time.Duration
	// MITM enables TLS interception for CONNECT when CA is set.
	MITM bool
	CA   *CA
	// Authorizer authenticates proxy clients; nil disables client auth.
	Authorizer Authorizer
}

// New returns a proxy server.
func New(mediator *mediation.Mediator) *Server {
	return &Server{Mediator: mediator, MaxBodyBytes: 1 << 20, DialTimeout: 10 * time.Second}
}

// ServeHTTP dispatches plain HTTP and CONNECT requests.
func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if s.Mediator == nil {
		http.Error(writer, "proxy not configured", http.StatusServiceUnavailable)
		return
	}
	if s.Authorizer != nil {
		if err := s.Authorizer.Authorize(request.Context(), request); err != nil {
			writer.Header().Set("Proxy-Authenticate", "Bearer")
			http.Error(writer, "proxy authentication required", http.StatusProxyAuthRequired)
			return
		}
	}
	if request.Method == http.MethodConnect {
		s.handleConnect(writer, request)
		return
	}
	s.handleHTTP(writer, request)
}

func (s *Server) handleHTTP(writer http.ResponseWriter, request *http.Request) {
	if !request.URL.IsAbs() {
		http.Error(writer, "proxy requests must use an absolute URL", http.StatusBadRequest)
		return
	}
	scheme := strings.ToLower(request.URL.Scheme)
	if scheme != "http" && scheme != "https" {
		http.Error(writer, "unsupported scheme", http.StatusBadRequest)
		return
	}
	port := 80
	if scheme == "https" {
		port = 443
	}
	if explicit := request.URL.Port(); explicit != "" {
		parsed, err := strconv.Atoi(explicit)
		if err != nil || parsed <= 0 || parsed > 65535 {
			http.Error(writer, "bad port", http.StatusBadRequest)
			return
		}
		port = parsed
	}
	maxBytes := s.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, int64(maxBytes)+1))
	if err != nil {
		http.Error(writer, "cannot read body", http.StatusBadRequest)
		return
	}
	if len(body) > maxBytes {
		http.Error(writer, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	query := map[string]string{}
	for key, values := range request.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}
	response, err := s.Mediator.Do(request.Context(), mediation.Request{
		Scheme: scheme, Protocol: "rest", Host: request.URL.Hostname(), Port: uint16(port),
		Method: request.Method, Path: request.URL.Path, Query: query, Body: body, Headers: forwardHeaders(request.Header),
		Credential:       credentials.Handle(request.Header.Get("X-Quarshell-Credential")),
		CredentialHeader: request.Header.Get("X-Quarshell-Credential-Header"),
		CredentialPrefix: request.Header.Get("X-Quarshell-Credential-Prefix"),
	})
	if err != nil {
		writeError(writer, err)
		return
	}
	for key, value := range response.Headers {
		writer.Header().Set(key, value)
	}
	writer.WriteHeader(response.Status)
	_, _ = writer.Write(response.Body)
}

func (s *Server) handleConnect(writer http.ResponseWriter, request *http.Request) {
	host, portText, err := net.SplitHostPort(request.Host)
	if err != nil {
		host, portText = request.Host, "443"
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		http.Error(writer, "bad CONNECT target", http.StatusBadRequest)
		return
	}
	if s.MITM && s.CA == nil {
		http.Error(writer, "TLS interception CA is not configured", http.StatusInternalServerError)
		return
	}
	if !s.MITM {
		if err := s.Mediator.Evaluate(request.Context(), mediation.Request{Protocol: "tcp", Host: host, Port: uint16(port)}); err != nil {
			writeError(writer, err)
			return
		}
	}
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "hijacking is unsupported", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	if s.MITM {
		_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		s.serveMITM(client, host, portText)
		return
	}
	timeout := s.DialTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	upstream, err := (&net.Dialer{Timeout: timeout}).DialContext(request.Context(), "tcp", net.JoinHostPort(host, portText))
	if err != nil {
		_, _ = io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer upstream.Close()
	_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
	relay(client, upstream)
}

func relay(client, upstream net.Conn) {
	done := make(chan struct{}, 2)
	copyDirection := func(destination, source net.Conn) {
		_, _ = io.Copy(destination, source)
		if tcp, ok := destination.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}
	go copyDirection(upstream, client)
	go copyDirection(client, upstream)
	<-done
	_ = client.Close()
	_ = upstream.Close()
	<-done
}

func forwardHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if isHopByHop(key) || len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	return result
}

func isHopByHop(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "X-Quarshell-Proxy-Token",
		"X-Quarshell-Credential", "X-Quarshell-Credential-Header", "X-Quarshell-Credential-Prefix",
		"Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func writeError(writer http.ResponseWriter, err error) {
	switch domain.CodeOf(err) {
	case domain.CodePermissionDenied, domain.CodeUnauthorized:
		http.Error(writer, "egress denied", http.StatusForbidden)
	case domain.CodeInvalid:
		http.Error(writer, "invalid request", http.StatusBadRequest)
	case domain.CodeUnsupported:
		http.Error(writer, "policy unsupported", http.StatusNotImplemented)
	case domain.CodeCanceled:
		http.Error(writer, "request canceled", http.StatusRequestTimeout)
	default:
		http.Error(writer, "upstream failure", http.StatusBadGateway)
	}
}

func (s *Server) serveMITM(client net.Conn, host, portText string) {
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return
	}
	leaf, err := s.CA.CertificateFor(host)
	if err != nil {
		return
	}
	server := tls.Server(client, &tls.Config{
		Certificates: []tls.Certificate{leaf},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"http/1.1"},
	})
	handshakeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.HandshakeContext(handshakeCtx); err != nil {
		_ = server.Close()
		return
	}
	defer server.Close()
	reader := bufio.NewReader(server)
	maxBytes := s.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	for {
		request, err := http.ReadRequest(reader)
		if err != nil {
			return
		}
		requestHost := request.Host
		if h, _, err := net.SplitHostPort(requestHost); err == nil {
			requestHost = h
		}
		if !strings.EqualFold(requestHost, host) {
			_ = writeMITMResponse(server, http.StatusBadRequest, []byte("host mismatch\n"), nil)
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, int64(maxBytes)+1))
		if err != nil {
			return
		}
		if len(body) > maxBytes {
			_ = writeMITMResponse(server, http.StatusRequestEntityTooLarge, []byte("request body too large\n"), nil)
			return
		}
		query := map[string]string{}
		for key, values := range request.URL.Query() {
			if len(values) > 0 {
				query[key] = values[0]
			}
		}
		response, err := s.Mediator.Do(request.Context(), mediation.Request{
			Scheme: "https", Protocol: "rest", Host: host, Port: uint16(port),
			Method: request.Method, Path: request.URL.Path, Query: query, Body: body, Headers: forwardHeaders(request.Header),
			Credential:       credentials.Handle(request.Header.Get("X-Quarshell-Credential")),
			CredentialHeader: request.Header.Get("X-Quarshell-Credential-Header"),
			CredentialPrefix: request.Header.Get("X-Quarshell-Credential-Prefix"),
		})
		if err != nil {
			status := mitmStatus(err)
			_ = writeMITMResponse(server, status, []byte(http.StatusText(status)+"\n"), nil)
			return
		}
		if err := writeMITMResponse(server, response.Status, response.Body, response.Headers); err != nil {
			return
		}
		if request.Close {
			return
		}
	}
}

func writeMITMResponse(writer io.Writer, status int, body []byte, headers map[string]string) error {
	response := &http.Response{
		StatusCode:    status,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	for key, value := range headers {
		if isHopByHop(key) {
			continue
		}
		response.Header.Set(key, value)
	}
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return response.Write(writer)
}

func mitmStatus(err error) int {
	switch domain.CodeOf(err) {
	case domain.CodePermissionDenied, domain.CodeUnauthorized:
		return http.StatusForbidden
	case domain.CodeInvalid:
		return http.StatusBadRequest
	case domain.CodeUnsupported:
		return http.StatusNotImplemented
	case domain.CodeCanceled:
		return http.StatusRequestTimeout
	default:
		return http.StatusBadGateway
	}
}
