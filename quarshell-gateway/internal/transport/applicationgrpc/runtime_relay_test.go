package applicationgrpc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	sandboxapp "github.com/KLORQ/quarshell-gateway/internal/application/sandbox"
	"github.com/KLORQ/quarshell-gateway/internal/sshrelay"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc"
)

type relayServerStream struct {
	grpc.ServerStream
	ctx       context.Context
	first     *gatewayv1.RelayFrame
	firstRead bool
	sent      chan *gatewayv1.RelayFrame
}

func (stream *relayServerStream) Context() context.Context { return stream.ctx }
func (stream *relayServerStream) Send(frame *gatewayv1.RelayFrame) error {
	select {
	case stream.sent <- frame:
		return nil
	case <-stream.ctx.Done():
		return stream.ctx.Err()
	}
}
func (stream *relayServerStream) Recv() (*gatewayv1.RelayFrame, error) {
	if !stream.firstRead {
		stream.firstRead = true
		return stream.first, nil
	}
	<-stream.ctx.Done()
	return nil, stream.ctx.Err()
}

func TestRelayStreamReturnsWhenGuestTargetCloses(t *testing.T) {
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()
	ref := domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}
	if _, _, err := sandboxes.Create(ctx, sandboxapp.Request{Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "image"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, _, err := sandboxes.Start(ctx, ref, 1, "start-1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	hub := sshrelay.NewHub()
	opened := make(chan string, 1)
	done := make(chan struct{})
	unregister, err := hub.RegisterSupervisor("sb", "instance-1", 1, func(channel, _ string) error {
		opened <- channel
		return nil
	}, done, nil)
	if err != nil {
		t.Fatalf("RegisterSupervisor: %v", err)
	}
	defer unregister()

	clientResult := make(chan net.Conn, 1)
	clientErr := make(chan error, 1)
	go func() {
		conn, err := hub.OpenChannel(ctx, "sb", "tcp://127.0.0.1:8080")
		if err != nil {
			clientErr <- err
			return
		}
		clientResult <- conn
	}()
	var channel string
	select {
	case channel = <-opened:
	case <-time.After(time.Second):
		t.Fatal("supervisor was not asked to open a relay")
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	streamCtx = context.WithValue(streamCtx, principalContextKey{}, authenticatedPrincipal{
		identityProvider: "sandbox-token", workspace: ref.Workspace, sandbox: ref.Sandbox, generation: 2, credentialEpoch: 2,
	})
	stream := &relayServerStream{
		ctx: streamCtx,
		first: &gatewayv1.RelayFrame{Payload: &gatewayv1.RelayFrame_Init{Init: &gatewayv1.RelayInit{
			ChannelId: channel,
		}}},
		sent: make(chan *gatewayv1.RelayFrame, 1),
	}
	server := &Server{Sandboxes: sandboxes, SupervisorHub: hub}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.RelayStream(stream) }()
	var client net.Conn
	select {
	case client = <-clientResult:
	case err := <-clientErr:
		t.Fatalf("OpenChannel: %v", err)
	case <-time.After(time.Second):
		t.Fatal("relay channel was not accepted")
	}
	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatalf("write relay data: %v", err)
	}
	select {
	case frame := <-stream.sent:
		if got := string(frame.GetData()); got != "hello" {
			t.Fatalf("relay data = %q, want hello", got)
		}
	case <-time.After(time.Second):
		t.Fatal("relay data was not forwarded")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close relay client: %v", err)
	}
	select {
	case err := <-serverErr:
		if err != nil && err != io.EOF {
			t.Fatalf("RelayStream after guest close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RelayStream did not finish after guest target closed")
	}
}
