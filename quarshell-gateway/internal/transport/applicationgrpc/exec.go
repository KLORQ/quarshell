package applicationgrpc

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/KLORQ/quarshell-driver/driver"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	execOutputChunkBytes = 32 * 1024
	defaultExecColumns   = 80
	defaultExecRows      = 24
)

// ExecSandbox executes one command and sends bounded stdout/stderr chunks plus
// a final exit event. Durable request_id admission is not yet supported.
func (s *Server) ExecSandbox(request *gatewayv1.ExecSandboxRequest, stream gatewayv1.OpenShell_ExecSandboxServer) error {
	if s.Sandboxes == nil {
		return status.Error(codes.Unavailable, "sandbox service is not configured")
	}
	if strings.TrimSpace(request.GetSandbox()) == "" || len(request.GetCommand()) == 0 {
		return status.Error(codes.InvalidArgument, "sandbox and command are required")
	}
	if strings.TrimSpace(request.GetRequestId()) != "" {
		return status.Error(codes.Unimplemented, "durable exec request_id admission is not configured")
	}
	if strings.TrimSpace(request.GetCommand()[0]) == "" || strings.ContainsRune(request.GetWorkdir(), '\x00') {
		return status.Error(codes.InvalidArgument, "command and workdir must be valid")
	}
	for _, argument := range request.GetCommand() {
		if strings.ContainsRune(argument, '\x00') {
			return status.Error(codes.InvalidArgument, "command contains an invalid argument")
		}
	}
	columns, rows := request.GetCols(), request.GetRows()
	if !request.GetTty() && (columns != 0 || rows != 0) {
		return status.Error(codes.InvalidArgument, "terminal dimensions require tty=true")
	}
	if request.GetTty() {
		if columns == 0 {
			columns = defaultExecColumns
		}
		if rows == 0 {
			rows = defaultExecRows
		}
	}
	ctx := stream.Context()
	if timeout := request.GetExecutionTimeout(); timeout != nil {
		if err := timeout.CheckValid(); err != nil || timeout.AsDuration() <= 0 {
			return status.Error(codes.InvalidArgument, "execution_timeout must be a positive duration")
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout.AsDuration())
		defer cancel()
	}
	environment := make([]string, 0, len(request.GetEnvironment()))
	keys := make([]string, 0, len(request.GetEnvironment()))
	for key := range request.GetEnvironment() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(request.GetEnvironment()[key], '\x00') {
			return status.Error(codes.InvalidArgument, "environment contains an invalid entry")
		}
		environment = append(environment, key+"="+request.GetEnvironment()[key])
	}
	ref := sandboxRef(request.GetWorkspaceScope(), request.GetSandbox())
	result, err := s.Sandboxes.Exec(ctx, ref, driver.ExecRequest{
		CaptureOutput: true,
		Argv:          append([]string(nil), request.GetCommand()...), TTY: request.GetTty(),
		Columns: columns, Rows: rows,
		Env: environment, WorkDir: request.GetWorkdir(), NoLoginShell: request.GetNoLoginShell(),
		Stdin: append([]byte(nil), request.GetStdin()...),
	})
	if err != nil {
		return grpcError(err)
	}
	if err := sendExecOutput(stream, result.Stdout, true); err != nil {
		return err
	}
	if err := sendExecOutput(stream, result.Stderr, false); err != nil {
		return err
	}
	return stream.Send(&gatewayv1.ExecSandboxEvent{Payload: &gatewayv1.ExecSandboxEvent_Exit{Exit: &gatewayv1.ExecSandboxExit{ExitCode: int32(result.ExitCode)}}})
}

// ExecSandboxInteractive relays stdin, terminal resizes, and PTY output for a
// command. Durable request_id admission is intentionally rejected until the
// execution fence is backed by a durable store.
func (s *Server) ExecSandboxInteractive(stream gatewayv1.OpenShell_ExecSandboxInteractiveServer) error {
	if s.Sandboxes == nil {
		return status.Error(codes.Unavailable, "sandbox service is not configured")
	}
	first, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return status.Error(codes.InvalidArgument, "interactive exec requires a start message")
		}
		return err
	}
	request := first.GetStart()
	if request == nil || strings.TrimSpace(request.GetSandbox()) == "" || len(request.GetCommand()) == 0 {
		return status.Error(codes.InvalidArgument, "interactive exec requires a valid start message")
	}
	if strings.TrimSpace(request.GetRequestId()) != "" {
		return status.Error(codes.Unimplemented, "durable exec request_id admission is not configured")
	}
	if !request.GetTty() {
		return status.Error(codes.InvalidArgument, "interactive exec requires tty=true")
	}
	if strings.TrimSpace(request.GetCommand()[0]) == "" || strings.ContainsRune(request.GetWorkdir(), '\x00') {
		return status.Error(codes.InvalidArgument, "command and workdir must be valid")
	}
	for _, argument := range request.GetCommand() {
		if strings.ContainsRune(argument, '\x00') {
			return status.Error(codes.InvalidArgument, "command contains an invalid argument")
		}
	}
	columns, rows := request.GetCols(), request.GetRows()
	if columns == 0 {
		columns = defaultExecColumns
	}
	if rows == 0 {
		rows = defaultExecRows
	}
	if columns > 65535 || rows > 65535 {
		return status.Error(codes.InvalidArgument, "terminal dimensions must not exceed 65535")
	}
	ctx := stream.Context()
	if timeout := request.GetExecutionTimeout(); timeout != nil {
		if err := timeout.CheckValid(); err != nil || timeout.AsDuration() <= 0 {
			return status.Error(codes.InvalidArgument, "execution_timeout must be a positive duration")
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout.AsDuration())
		defer cancel()
	}
	environment, err := execEnvironment(request.GetEnvironment())
	if err != nil {
		return err
	}
	session, err := s.Sandboxes.OpenExecTerminal(ctx, sandboxRef(request.GetWorkspaceScope(), request.GetSandbox()), driver.ExecTerminalRequest{
		Argv: append([]string(nil), request.GetCommand()...), Env: environment,
		WorkDir: request.GetWorkdir(), NoLoginShell: request.GetNoLoginShell(), Columns: columns, Rows: rows,
	})
	if err != nil {
		return grpcError(err)
	}
	defer session.Close()
	if len(request.GetStdin()) > 0 {
		if _, err := session.Write(request.GetStdin()); err != nil {
			return grpcError(err)
		}
	}
	inputErrors := make(chan error, 1)
	go func() {
		for {
			input, err := stream.Recv()
			if err == io.EOF {
				if closer, ok := session.(driver.ExecTerminalInputCloser); ok {
					err = closer.CloseStdin()
				}
				inputErrors <- err
				return
			}
			if err != nil {
				inputErrors <- err
				_ = session.Close()
				return
			}
			switch payload := input.GetPayload().(type) {
			case *gatewayv1.ExecSandboxInput_Stdin:
				if _, err := session.Write(payload.Stdin); err != nil {
					inputErrors <- err
					_ = session.Close()
					return
				}
			case *gatewayv1.ExecSandboxInput_Resize:
				if payload.Resize == nil || payload.Resize.GetCols() == 0 || payload.Resize.GetRows() == 0 || payload.Resize.GetCols() > 65535 || payload.Resize.GetRows() > 65535 {
					inputErrors <- status.Error(codes.InvalidArgument, "terminal resize dimensions must be between 1 and 65535")
					_ = session.Close()
					return
				}
				if err := session.Resize(ctx, payload.Resize.GetCols(), payload.Resize.GetRows()); err != nil {
					inputErrors <- grpcError(err)
					_ = session.Close()
					return
				}
			default:
				inputErrors <- status.Error(codes.InvalidArgument, "interactive exec accepts only stdin and resize after start")
				_ = session.Close()
				return
			}
		}
	}()
	buffer := make([]byte, execOutputChunkBytes)
	for {
		count, readErr := session.Read(buffer)
		if count > 0 {
			if err := stream.Send(&gatewayv1.ExecSandboxEvent{Payload: &gatewayv1.ExecSandboxEvent_Stdout{Stdout: &gatewayv1.ExecSandboxStdout{Data: append([]byte(nil), buffer[:count]...)}}}); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			select {
			case inputErr := <-inputErrors:
				if inputErr != nil && inputErr != io.EOF {
					return inputErr
				}
			default:
			}
			return grpcError(readErr)
		}
	}
	exitCode, err := session.Wait(ctx)
	if err != nil {
		return grpcError(err)
	}
	return stream.Send(&gatewayv1.ExecSandboxEvent{Payload: &gatewayv1.ExecSandboxEvent_Exit{Exit: &gatewayv1.ExecSandboxExit{ExitCode: int32(exitCode)}}})
}

func execEnvironment(values map[string]string) ([]string, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(values[key], '\x00') {
			return nil, status.Error(codes.InvalidArgument, "environment contains an invalid entry")
		}
		environment = append(environment, key+"="+values[key])
	}
	return environment, nil
}

func sendExecOutput(stream gatewayv1.OpenShell_ExecSandboxServer, output []byte, stdout bool) error {
	for len(output) > 0 {
		chunk := output
		if len(chunk) > execOutputChunkBytes {
			chunk = chunk[:execOutputChunkBytes]
		}
		payload := &gatewayv1.ExecSandboxEvent_Stdout{Stdout: &gatewayv1.ExecSandboxStdout{Data: append([]byte(nil), chunk...)}}
		if !stdout {
			payload = nil
			if err := stream.Send(&gatewayv1.ExecSandboxEvent{Payload: &gatewayv1.ExecSandboxEvent_Stderr{Stderr: &gatewayv1.ExecSandboxStderr{Data: append([]byte(nil), chunk...)}}}); err != nil {
				return err
			}
		} else if err := stream.Send(&gatewayv1.ExecSandboxEvent{Payload: payload}); err != nil {
			return err
		}
		output = output[len(chunk):]
	}
	return nil
}
