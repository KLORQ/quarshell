//go:build darwin && arm64

package applicationgrpc

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/KLORQ/quarshell-core"
	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-driver-vm/vm"
	"github.com/KLORQ/quarshell-driver/driver"
	"github.com/KLORQ/quarshell-gateway/internal/application/lifecycle"
	sandboxapp "github.com/KLORQ/quarshell-gateway/internal/application/sandbox"
	"github.com/KLORQ/quarshell-gateway/internal/application/sandboxlogs"
	"github.com/KLORQ/quarshell-gateway/internal/application/sandboxruntime"
	dm "github.com/KLORQ/quarshell-proto/gen/go/openshell/datamodel/v1"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type liveVMWatchRuntime struct {
	backend   *vm.Driver
	rootfsTar string
}

func (r *liveVMWatchRuntime) Create(ctx context.Context, sandbox sandboxruntime.Sandbox) (string, error) {
	request := driver.BackendCreateRequest{
		SandboxID: core.ID(string(sandbox.Sandbox)), Name: string(sandbox.Sandbox),
		Workspace: driver.WorkspaceTarget{Kind: driver.WorkspaceGuestOverlay, ID: string(sandbox.Workspace), Persist: true},
		Workload:  driver.WorkloadSpec{Image: sandbox.Image, Command: append([]string(nil), sandbox.Command...), TTY: sandbox.TTY},
		Runtime:   driver.RuntimeAssets{},
		Config:    vm.SandboxDriverConfig{RootfsTarPath: r.rootfsTar},
	}
	handle, err := r.backend.Create(ctx, request)
	return string(handle.ID), err
}

func (r *liveVMWatchRuntime) Start(ctx context.Context, sandbox sandboxruntime.Sandbox) error {
	return r.backend.Start(ctx, core.ID(sandbox.RuntimeID))
}
func (r *liveVMWatchRuntime) Stop(ctx context.Context, sandbox sandboxruntime.Sandbox) error {
	return r.backend.Stop(ctx, core.ID(sandbox.RuntimeID))
}
func (r *liveVMWatchRuntime) Delete(ctx context.Context, sandbox sandboxruntime.Sandbox) error {
	return r.backend.Delete(ctx, core.ID(sandbox.RuntimeID))
}
func (r *liveVMWatchRuntime) Inspect(ctx context.Context, sandbox sandboxruntime.Sandbox) (driver.Info, error) {
	return r.backend.Inspect(ctx, sandbox.RuntimeID)
}
func (r *liveVMWatchRuntime) WatchPlatformEvents(ctx context.Context, sandbox sandboxruntime.Sandbox) (<-chan driver.PlatformEventResult, error) {
	return r.backend.WatchPlatformEvents(ctx, driver.PlatformEventWatchRequest{SandboxID: core.ID(sandbox.RuntimeID), SandboxName: string(sandbox.Sandbox)})
}

func (r *liveVMWatchRuntime) OpenExecStream(ctx context.Context, sandbox sandboxruntime.Sandbox, request driver.ExecTerminalRequest) (driver.ExecStreamSession, error) {
	return r.backend.OpenExecStream(ctx, core.ID(sandbox.RuntimeID), request)
}

func TestLiveVMWatchSandboxComposition(t *testing.T) {
	if os.Getenv("CAUTEM_VM_ACCEPTANCE") != "1" {
		t.Skip("set CAUTEM_VM_ACCEPTANCE=1 to run live HVF Gateway watch acceptance")
	}
	rootfsTar := strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_ROOTFS_TAR"))
	if rootfsTar == "" {
		t.Fatal("CAUTEM_VM_TEST_ROOTFS_TAR is required")
	}
	host := vm.DefaultHostConfig()
	host.StateDir = t.TempDir()
	host.RootfsTarDir = filepath.Join(host.StateDir, "rootfs-tar-staging")
	requestDir := filepath.Join(host.RootfsTarDir, "live-watch-request")
	if err := os.MkdirAll(requestDir, 0o700); err != nil {
		t.Fatal(err)
	}
	stagedRootfsTar := filepath.Join(requestDir, "rootfs.tar")
	sourceRootfs, err := os.Open(rootfsTar)
	if err != nil {
		t.Fatal(err)
	}
	stagedRootfs, err := os.OpenFile(stagedRootfsTar, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = sourceRootfs.Close()
		t.Fatal(err)
	}
	_, copyErr := io.Copy(stagedRootfs, sourceRootfs)
	if err := errors.Join(copyErr, stagedRootfs.Close(), sourceRootfs.Close()); err != nil {
		t.Fatalf("stage live VM rootfs tar: %v", err)
	}
	host.GuestAgentPath = strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_GUEST_AGENT"))
	host.WorkerPath = strings.TrimSpace(os.Getenv("CAUTEM_VM_WORKER"))
	host.FormatterPath = strings.TrimSpace(os.Getenv("CAUTEM_VM_TEST_E2FS_FORMATTER"))
	host.AllowDriverConfig = true
	// The driver needs a root-capable bootstrap environment to prepare the
	// workload rootfs disk; the acceptance script pre-pulls this local image.
	host.BootstrapImage = "docker.io/library/alpine:3.22"
	host.DefaultImage = "docker.io/library/alpine:3.22"
	backend := vm.NewWithHostConfig(host)
	runtime := &liveVMWatchRuntime{backend: backend, rootfsTar: stagedRootfsTar}
	store := lifecycle.NewMemoryStore()
	sandboxes, err := sandboxapp.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	sandboxes.Runtime = runtime
	server := New("live-vm-watch", nil, sandboxes, nil, nil, nil, nil, nil)
	logs, err := sandboxlogs.New(store)
	if err != nil {
		t.Fatal(err)
	}
	server.SandboxLogs = logs
	ref := domain.SandboxRef{Workspace: "live-vm-watch", Sandbox: "live-vm-watch"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	created, _, err := sandboxes.Create(ctx, sandboxapp.Request{
		Workspace: ref.Workspace, Sandbox: ref.Sandbox, Generation: 1, Image: "alpine:3.22",
		Command:          []string{"/bin/sh", "-c", "while :; do sleep 3600; done"},
		DriverConfigJSON: fmt.Sprintf(`{"vm":{"rootfs_tar_path":%q}}`, stagedRootfsTar),
		IdempotencyKey:   "live-vm-watch-create",
	})
	if err != nil {
		t.Fatalf("create live VM through Gateway sandbox service: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		if current, getErr := sandboxes.Get(cleanupCtx, ref); getErr == nil && current.State == "running" {
			_, _, _ = sandboxes.Stop(cleanupCtx, ref, current.Generation, "live-vm-watch-cleanup-stop")
		}
		current, getErr := sandboxes.Get(cleanupCtx, ref)
		if getErr == nil {
			_ = runtime.Delete(cleanupCtx, sandboxruntime.Sandbox{Sandbox: current.Sandbox, RuntimeID: current.RuntimeID})
		}
	})
	if created.RuntimeID == "" {
		t.Fatal("Gateway sandbox service did not retain the VM runtime ID")
	}
	watchCtx, stopWatch := context.WithCancel(ctx)
	stream := &captureSandboxWatchStream{ctx: watchCtx, events: make(chan *gatewayv1.SandboxStreamEvent, 16)}
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- server.WatchSandbox(&gatewayv1.WatchSandboxRequest{
			WorkspaceScope: &dm.WorkspaceSelector{Selection: &dm.WorkspaceSelector_Workspace{Workspace: string(ref.Workspace)}},
			Sandbox:        string(ref.Sandbox), FollowEvents: true, StopOnTerminal: true,
		}, stream)
	}()
	select {
	case event := <-stream.events:
		if event.GetSandbox() == nil {
			t.Fatalf("Gateway watch initial snapshot=%v", event)
		}
	case <-ctx.Done():
		t.Fatalf("waiting for Gateway watch snapshot: %v", ctx.Err())
	}
	started, _, err := sandboxes.Start(ctx, ref, created.Generation, "live-vm-watch-start")
	if err != nil {
		t.Fatalf("start VM through Gateway sandbox service: %v", err)
	}
	seenRunning := false
	for !seenRunning {
		select {
		case event := <-stream.events:
			platform := event.GetEvent()
			if platform != nil && platform.GetSource() == "vm" && platform.GetReason() == "running" {
				seenRunning = true
			}
		case <-ctx.Done():
			t.Fatalf("waiting for live VM running event through Gateway WatchSandbox (runtime=%s, state=%s): %v", started.RuntimeID, started.State, ctx.Err())
		}
	}
	assertLiveVMArchiveTransferThroughGateway(t, server, ref, ctx)
	stopped, _, err := sandboxes.Stop(ctx, ref, started.Generation, "live-vm-watch-stop")
	if err != nil {
		t.Fatalf("stop VM through Gateway sandbox service: %v", err)
	}
	seenStopped := false
	for !seenStopped {
		select {
		case event := <-stream.events:
			platform := event.GetEvent()
			if platform != nil && platform.GetSource() == "vm" && platform.GetReason() == "stopped" {
				seenStopped = true
			}
		case <-ctx.Done():
			t.Fatalf("waiting for live VM stopped event through Gateway WatchSandbox (state=%s): %v", stopped.State, ctx.Err())
		}
	}
	stopWatch()
	if err := <-watchDone; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Gateway WatchSandbox returned %v", err)
	}
}

func assertLiveVMArchiveTransferThroughGateway(t *testing.T, server *Server, ref domain.SandboxRef, ctx context.Context) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	gatewayv1.RegisterOpenShellServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})
	connection, err := grpc.NewClient("passthrough:///live-vm-gateway",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("connect to live Gateway gRPC fixture: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := gatewayv1.NewOpenShellClient(connection)
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	entries := []struct {
		name, body string
		kind       byte
		link       string
	}{
		{name: "gateway-transfer/nested/payload.txt", body: "gateway-live-vm-archive"},
		{name: "gateway-transfer/nested/relative-link", kind: tar.TypeSymlink, link: "payload.txt"},
	}
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o640, Typeflag: tar.TypeReg, Size: int64(len(entry.body))}
		if entry.kind != 0 {
			header.Typeflag, header.Linkname, header.Size = entry.kind, entry.link, 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if entry.kind == 0 {
			if _, err := writer.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	selector := &dm.WorkspaceSelector{Selection: &dm.WorkspaceSelector_Workspace{Workspace: string(ref.Workspace)}}
	run := func(requestID string, command []string, input []byte) []*gatewayv1.ExecSandboxEvent {
		stream, err := client.ExecSandboxInteractive(ctx)
		if err != nil {
			t.Fatalf("open live Gateway exec stream: %v", err)
		}
		if err := stream.Send(&gatewayv1.ExecSandboxInput{Payload: &gatewayv1.ExecSandboxInput_Start{Start: &gatewayv1.ExecSandboxRequest{
			WorkspaceScope: selector, Sandbox: string(ref.Sandbox), Command: command, RequestId: requestID,
		}}}); err != nil {
			t.Fatalf("send live Gateway exec start: %v", err)
		}
		if input != nil {
			if err := stream.Send(&gatewayv1.ExecSandboxInput{Payload: &gatewayv1.ExecSandboxInput_Stdin{Stdin: input}}); err != nil {
				t.Fatalf("send live Gateway archive: %v", err)
			}
		}
		if err := stream.CloseSend(); err != nil {
			t.Fatalf("close live Gateway exec input: %v", err)
		}
		var events []*gatewayv1.ExecSandboxEvent
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				return events
			}
			if err != nil {
				t.Fatalf("receive live Gateway VM archive exec %v: %v", command, err)
			}
			events = append(events, event)
		}
	}
	uploadEvents := run("3f655244-c0a7-4a2e-8125-2ee06d0e0423", []string{"/bin/tar", "-xf", "-", "-C", "/sandbox"}, archive.Bytes())
	if len(uploadEvents) != 1 || uploadEvents[0].GetExit() == nil || uploadEvents[0].GetExit().GetExitCode() != 0 {
		t.Fatalf("Gateway archive upload events=%+v", uploadEvents)
	}
	downloadEvents := run("f2f8396d-93db-48d9-8d82-47747d91bf8c", []string{"/bin/tar", "-cf", "-", "-C", "/sandbox", "gateway-transfer"}, nil)
	var downloaded bytes.Buffer
	for _, event := range downloadEvents {
		if stdout := event.GetStdout(); stdout != nil {
			_, _ = downloaded.Write(stdout.GetData())
		}
	}
	if len(downloadEvents) == 0 || downloadEvents[len(downloadEvents)-1].GetExit() == nil || downloadEvents[len(downloadEvents)-1].GetExit().GetExitCode() != 0 {
		t.Fatalf("Gateway archive download events=%+v", downloadEvents)
	}
	reader := tar.NewReader(bytes.NewReader(downloaded.Bytes()))
	seenFile, seenLink := false, false
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read downloaded Gateway VM archive: %v", err)
		}
		switch header.Name {
		case "gateway-transfer/nested/payload.txt":
			body, err := io.ReadAll(reader)
			if err != nil || string(body) != "gateway-live-vm-archive" {
				t.Fatalf("downloaded archive payload=%q err=%v", body, err)
			}
			seenFile = true
		case "gateway-transfer/nested/relative-link":
			if header.Typeflag != tar.TypeSymlink || header.Linkname != "payload.txt" {
				t.Fatalf("downloaded archive symlink=%+v", header)
			}
			seenLink = true
		}
	}
	if !seenFile || !seenLink {
		t.Fatalf("downloaded Gateway VM archive lost entries: file=%v symlink=%v", seenFile, seenLink)
	}
}

var _ sandboxruntime.Runtime = (*liveVMWatchRuntime)(nil)
var _ interface {
	WatchPlatformEvents(context.Context, sandboxruntime.Sandbox) (<-chan driver.PlatformEventResult, error)
} = (*liveVMWatchRuntime)(nil)
