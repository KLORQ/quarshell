package proxy

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-proxy/proxy/middleware"
)

type directionCaptureSession struct {
	mu         sync.Mutex
	directions []middleware.WebSocketDirection
	sequences  []uint64
	called     chan struct{}
}

func (s *directionCaptureSession) Preflight(context.Context) (bool, error) { return true, nil }
func (s *directionCaptureSession) Start(context.Context, string) error     { return nil }
func (s *directionCaptureSession) Close(string) error                      { return nil }
func (s *directionCaptureSession) Message(_ context.Context, sequence uint64, direction middleware.WebSocketDirection, payload []byte, _ bool) (middleware.WebSocketMessageDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.directions = append(s.directions, direction)
	s.sequences = append(s.sequences, sequence)
	s.called <- struct{}{}
	return middleware.WebSocketMessageDecision{Allow: true, Replacement: payload}, nil
}

func TestRelayWebSocketMiddlewareReceivesBothDirections(t *testing.T) {
	frame := func(opcode byte, payload []byte, masked bool) string {
		var b strings.Builder
		if err := writeWSFrame(&b, opcode, payload, masked); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	clientInput := frame(wsOpcodeBinary, []byte("request"), true) + frame(wsOpcodeClose, nil, true)
	upstreamReader, upstreamWriter := io.Pipe()
	go func() {
		defer upstreamWriter.Close()
		_, _ = io.WriteString(upstreamWriter, frame(wsOpcodeBinary, []byte("response"), false))
		_, _ = io.WriteString(upstreamWriter, frame(wsOpcodeClose, nil, false))
	}()
	session := &directionCaptureSession{called: make(chan struct{}, 2)}
	(&Server{}).relayWebsocket(context.Background(), bufio.NewReader(strings.NewReader(clientInput)), io.Discard, bufio.NewReader(upstreamReader), io.Discard, "example.test", 443, "/ws", nil, nil, nil, nil, false, "", false, "", []middleware.WebSocketSession{session})
	for range 2 {
		select {
		case <-session.called:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for both relay directions")
		}
	}
	session.mu.Lock()
	directions := append([]middleware.WebSocketDirection(nil), session.directions...)
	sequences := append([]uint64(nil), session.sequences...)
	session.mu.Unlock()
	if len(directions) != 2 {
		t.Fatalf("captured directions: %v", directions)
	}
	if len(directions) != 2 || !containsDirection(directions, middleware.WebSocketClientToUpstream) || !containsDirection(directions, middleware.WebSocketUpstreamToClient) {
		t.Fatalf("captured directions: %v", directions)
	}
	if sequences[0] == sequences[1] {
		t.Fatalf("concurrent relay messages reused sequence %d", sequences[0])
	}
}

func containsDirection(directions []middleware.WebSocketDirection, target middleware.WebSocketDirection) bool {
	for _, direction := range directions {
		if direction == target {
			return true
		}
	}
	return false
}
