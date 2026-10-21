package proxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/engine"
	"github.com/KLORQ/quarshell-core/policy"
	"github.com/KLORQ/quarshell-proxy/proxy/middleware"
)

type synchronizedCapture struct {
	mu sync.Mutex
	bytes.Buffer
	writes chan struct{}
}

func (w *synchronizedCapture) Write(data []byte) (int, error) {
	w.mu.Lock()
	n, err := w.Buffer.Write(data)
	w.mu.Unlock()
	w.writes <- struct{}{}
	return n, err
}

func (w *synchronizedCapture) snapshot() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.Buffer.Bytes()...)
}

func TestRegexBuiltinRedactsLiveHTTPProxyRequest(t *testing.T) {
	received := make(chan string, 2)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read origin request: %v", err)
			return
		}
		received <- string(body)
		_, _ = w.Write([]byte("ok"))
	}))
	defer origin.Close()
	originURL, err := url.Parse(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(originURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	doc := policy.Document{Version: 1}
	doc.SetNetworkAllows([]policy.AllowRule{{Host: "127.0.0.1", Port: port}})
	var eng engine.Allowlist
	if err := eng.Apply(doc); err != nil {
		t.Fatal(err)
	}
	var audit bytes.Buffer
	server := NewServerForTests(&eng, &audit)
	stage, err := middleware.NewRegexBuiltinStage(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	server.Middleware = &middleware.Pipeline{Stages: []middleware.Stage{stage}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() { _ = server.Serve(t.Context(), listener) }()
	proxyURL, err := url.Parse("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 3 * time.Second}

	for _, tc := range []struct{ input, want string }{
		{"payload sk-12345678901234567890", "payload [REDACTED]"},
		{"ordinary payload", "ordinary payload"},
	} {
		response, err := client.Post(origin.URL, "text/plain", strings.NewReader(tc.input))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("proxy response status=%d", response.StatusCode)
		}
		select {
		case got := <-received:
			if got != tc.want {
				t.Fatalf("origin received %q, want %q", got, tc.want)
			}
		case <-time.After(time.Second):
			t.Fatal("origin did not receive proxied request")
		}
	}
	lines := audit.String()
	if strings.Count(lines, "FINDING:CREATE") != 1 || !strings.Contains(lines, "regex.openai count:1") || !strings.Contains(lines, "regex_matches_replaced:1") {
		t.Fatalf("unexpected audit findings: %s", lines)
	}
	if strings.Contains(lines, "sk-12345678901234567890") {
		t.Fatalf("audit leaked the matched key: %s", lines)
	}
}

func TestRegexBuiltinRedactsOnlyClientWebSocketMessages(t *testing.T) {
	frame := func(opcode byte, payload []byte, masked bool) []byte {
		var output bytes.Buffer
		if err := writeWSFrame(&output, opcode, payload, masked); err != nil {
			t.Fatal(err)
		}
		return output.Bytes()
	}
	secret := []byte("sk-12345678901234567890")
	clientInput := append(frame(wsOpcodeText, secret, true), frame(wsOpcodeClose, nil, true)...)
	upstreamReader, upstreamWriter := io.Pipe()
	upstreamOutput := &synchronizedCapture{writes: make(chan struct{}, 4)}
	clientOutput := &synchronizedCapture{writes: make(chan struct{}, 4)}
	go func() {
		defer upstreamWriter.Close()
		_, _ = upstreamWriter.Write(frame(wsOpcodeText, secret, false))
		_, _ = upstreamWriter.Write(frame(wsOpcodeClose, nil, false))
	}()
	doc := policy.Document{Version: 1}
	doc.SetNetworkAllows([]policy.AllowRule{{
		Host: "api.example", Port: 443, Protocol: policy.ProtocolWebsocket,
		Rules: []policy.L7Rule{{Allow: &policy.L7Allow{Method: policy.MethodWebsocketText, Path: "/ws"}}},
	}})
	var eng engine.Allowlist
	if err := eng.Apply(doc); err != nil {
		t.Fatal(err)
	}
	stage, err := middleware.NewRegexBuiltinStage(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := stage.OpenWebSocketSession(context.Background(), middleware.WebSocketRequest{Host: "api.example", Port: 443, Path: "/ws"})
	if err != nil {
		t.Fatal(err)
	}
	(&Server{}).relayWebsocket(context.Background(), bufio.NewReader(bytes.NewReader(clientInput)), clientOutput, bufio.NewReader(upstreamReader), upstreamOutput, "api.example", 443, "/ws", nil, &eng, nil, nil, false, "", false, "", []middleware.WebSocketSession{session})
	waitForWrites := func(writer *synchronizedCapture) {
		t.Helper()
		for i := 0; i < 2; i++ {
			select {
			case <-writer.writes:
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for both WebSocket frames")
			}
		}
	}
	waitForWrites(upstreamOutput)
	waitForWrites(clientOutput)
	readText := func(raw []byte) []byte {
		t.Helper()
		frame, err := readWSFrame(bufio.NewReader(bytes.NewReader(raw)))
		if err != nil {
			t.Fatalf("decode WebSocket frame: %v", err)
		}
		if frame.Opcode != wsOpcodeText {
			t.Fatalf("first frame opcode=%d want text", frame.Opcode)
		}
		return frame.Payload
	}
	if got := readText(upstreamOutput.snapshot()); string(got) != "[REDACTED]" {
		t.Fatalf("upstream received %q, want redacted message", got)
	}
	if got := readText(clientOutput.snapshot()); string(got) != string(secret) {
		t.Fatalf("client received modified response %q", got)
	}
}
