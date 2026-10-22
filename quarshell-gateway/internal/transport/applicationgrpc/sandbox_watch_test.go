package applicationgrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	sandboxapp "github.com/KLORQ/quarshell-gateway/internal/application/sandbox"
	"github.com/KLORQ/quarshell-gateway/internal/application/sandboxlogs"
	dm "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc"
)

type captureSandboxWatchStream struct {
	grpc.ServerStream
	ctx    context.Context
	events chan *gatewayv1.SandboxStreamEvent
}

func (s *captureSandboxWatchStream) Context() context.Context { return s.ctx }
func (s *captureSandboxWatchStream) Send(event *gatewayv1.SandboxStreamEvent) error {
	select {
	case s.events <- event:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func TestWatchSandboxReplaysTailAndStreamsLiveSandboxLogs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}
	if _, _, err := sandboxes.Create(context.Background(), sandboxapp.Request{Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "image"}); err != nil {
		t.Fatal(err)
	}
	started, _, err := sandboxes.Start(context.Background(), ref, 1, "start")
	if err != nil {
		t.Fatal(err)
	}
	logs, err := sandboxlogs.New(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := logs.Append(context.Background(), "sb", []sandboxlogs.Line{{SandboxID: "sb", Source: "sandbox", Level: "INFO", Message: "older"}, {SandboxID: "sb", Source: "sandbox", Level: "INFO", Message: "tail"}}); err != nil {
		t.Fatal(err)
	}
	server := New("test", nil, sandboxes, nil, nil, nil, nil, nil)
	server.SandboxLogs = logs
	stream := &captureSandboxWatchStream{ctx: ctx, events: make(chan *gatewayv1.SandboxStreamEvent, 8)}
	request := &gatewayv1.WatchSandboxRequest{WorkspaceScope: &dm.WorkspaceSelector{Selection: &dm.WorkspaceSelector_Workspace{Workspace: "ws"}}, Sandbox: "sb", FollowLogs: true, LogTailLines: 1, LogSources: []string{"sandbox"}}
	done := make(chan error, 1)
	go func() { done <- server.WatchSandbox(request, stream) }()
	select {
	case event := <-stream.events:
		if event.GetSandbox() == nil {
			t.Fatalf("initial snapshot event=%v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial sandbox snapshot")
	}
	tailCursor := ""
	select {
	case event := <-stream.events:
		if line := event.GetLog(); line == nil || line.GetMessage() != "tail" {
			t.Fatalf("tail replay event=%v", event)
		}
		tailCursor = event.GetCursor()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for tail replay")
	}

	sandboxCtx := context.WithValue(context.Background(), principalContextKey{}, authenticatedPrincipal{
		identityProvider: "sandbox-token", sandbox: "sb", generation: started.RuntimeGeneration, credentialEpoch: started.CredentialEpoch,
	})
	push := &sandboxLogsStream{ctx: sandboxCtx, requests: []*gatewayv1.PushSandboxLogsRequest{{SandboxId: "sb", Logs: []*gatewayv1.SandboxLogLine{{SandboxId: "sb", Source: "sandbox", Level: "WARN", Message: "live"}}}}}
	if err := server.PushSandboxLogs(push); err != nil {
		t.Fatal(err)
	}
	liveCursor := ""
	select {
	case event := <-stream.events:
		if line := event.GetLog(); line == nil || line.GetMessage() != "live" {
			t.Fatalf("live event=%v", event)
		}
		liveCursor = event.GetCursor()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live log")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("watch stop error=%v", err)
	}

	resumeCtx, resumeCancel := context.WithCancel(context.Background())
	resumeStream := &captureSandboxWatchStream{ctx: resumeCtx, events: make(chan *gatewayv1.SandboxStreamEvent, 8)}
	resumeDone := make(chan error, 1)
	go func() {
		resumeDone <- server.WatchSandbox(&gatewayv1.WatchSandboxRequest{
			WorkspaceScope: request.GetWorkspaceScope(), Sandbox: "sb", FollowLogs: true, ResumeAfterCursor: tailCursor,
		}, resumeStream)
	}()
	select {
	case event := <-resumeStream.events:
		if event.GetSandbox() == nil {
			t.Fatalf("resume initial snapshot event=%v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resume sandbox snapshot")
	}
	select {
	case event := <-resumeStream.events:
		if line := event.GetLog(); line == nil || line.GetMessage() != "live" || event.GetCursor() != liveCursor {
			t.Fatalf("resumed event=%v cursor=%q want=%q", event, event.GetCursor(), liveCursor)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cursor replay")
	}
	resumeCancel()
	if err := <-resumeDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("resumed watch stop error=%v", err)
	}
}

func TestWatchSandboxStopsAtInitialTerminalPhase(t *testing.T) {
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}
	if _, _, err := sandboxes.Create(context.Background(), sandboxapp.Request{Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "image"}); err != nil {
		t.Fatal(err)
	}
	started, _, err := sandboxes.Start(context.Background(), ref, 1, "start")
	if err != nil {
		t.Fatal(err)
	}
	if err := sandboxes.RecordMainProcessExit(context.Background(), ref, started.RuntimeGeneration, "instance", 0); err != nil {
		t.Fatal(err)
	}
	server := New("test", nil, sandboxes, nil, nil, nil, nil, nil)
	stream := &captureSandboxWatchStream{ctx: context.Background(), events: make(chan *gatewayv1.SandboxStreamEvent, 1)}
	err = server.WatchSandbox(&gatewayv1.WatchSandboxRequest{
		WorkspaceScope: &dm.WorkspaceSelector{Selection: &dm.WorkspaceSelector_Workspace{Workspace: "ws"}},
		Sandbox:        "sb", FollowStatus: true, StopOnTerminal: true,
	}, stream)
	if err != nil {
		t.Fatal(err)
	}
	event := <-stream.events
	if got := event.GetSandbox().GetStatus().GetPhase(); got != gatewayv1.SandboxPhase_SANDBOX_PHASE_COMPLETED {
		t.Fatalf("terminal watch phase=%s", got)
	}
}
